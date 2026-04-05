package service

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"lazyops-server/internal/models"
)

type HealthCheckProbeConfig struct {
	Protocol         string
	Host             string
	Port             int
	Path             string
	Timeout          time.Duration
	SuccessThreshold int
	FailureThreshold int
	RetryInterval    time.Duration
}

type ProductionHealthGateEvaluator struct {
	instances  InstanceStore
	revisions  DesiredStateRevisionStore
	bindings   DeploymentBindingStore
	httpClient *http.Client
	defaultCfg HealthCheckProbeConfig
}

func NewProductionHealthGateEvaluator(
	instances InstanceStore,
	revisions DesiredStateRevisionStore,
	bindings DeploymentBindingStore,
) *ProductionHealthGateEvaluator {
	return &ProductionHealthGateEvaluator{
		instances:  instances,
		revisions:  revisions,
		bindings:   bindings,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		defaultCfg: HealthCheckProbeConfig{
			Timeout:          5 * time.Second,
			SuccessThreshold: 1,
			FailureThreshold: 3,
			RetryInterval:    2 * time.Second,
		},
	}
}

func (e *ProductionHealthGateEvaluator) Evaluate(ctx context.Context, projectID, deploymentID, revisionID string) (*HealthGateResult, error) {
	revision, err := e.revisions.GetByIDForProject(projectID, revisionID)
	if err != nil {
		return nil, err
	}
	if revision == nil {
		return nil, ErrRevisionNotFound
	}

	compiled, err := parseCompiledRevision(revision.CompiledRevisionJSON)
	if err != nil {
		return nil, fmt.Errorf("parse compiled revision: %w", err)
	}

	instance, err := e.resolveInstanceForRevision(ctx, projectID, compiled)
	if err != nil {
		return nil, fmt.Errorf("resolve instance for health gate: %w", err)
	}

	targetIP := resolveTargetIP(instance)
	if targetIP == "" {
		return nil, fmt.Errorf("no routable IP for instance %q", instance.ID)
	}

	result := &HealthGateResult{
		RevisionID:   revisionID,
		DeploymentID: deploymentID,
		Services:     make([]ServiceHealthResult, 0, len(compiled.Services)),
	}

	allPassed := true
	for _, svc := range compiled.Services {
		hc := svc.Healthcheck
		if hc == nil {
			hc = map[string]any{}
		}

		cfg := e.buildProbeConfig(hc, targetIP)
		probeResult := e.probeService(ctx, cfg, svc.Name)

		result.Services = append(result.Services, ServiceHealthResult{
			ServiceName: svc.Name,
			Healthy:     probeResult.Passed,
			Healthcheck: hc,
			Message:     probeResult.Message,
		})

		if !probeResult.Passed {
			allPassed = false
		}
	}

	result.Passed = allPassed
	return result, nil
}

func (e *ProductionHealthGateEvaluator) resolveInstanceForRevision(ctx context.Context, projectID string, compiled desiredStateRevisionCompiledRecord) (*models.Instance, error) {
	binding, err := e.bindings.GetByIDForProject(projectID, compiled.DeploymentBindingID)
	if err != nil {
		return nil, fmt.Errorf("lookup deployment binding: %w", err)
	}
	if binding == nil {
		return nil, fmt.Errorf("deployment binding %q not found", compiled.DeploymentBindingID)
	}
	if binding.TargetKind != "instance" {
		return nil, fmt.Errorf("health gate only supports instance targets, got %q", binding.TargetKind)
	}

	instance, err := e.instances.GetByID(binding.TargetID)
	if err != nil {
		return nil, fmt.Errorf("lookup instance %q: %w", binding.TargetID, err)
	}
	if instance == nil {
		return nil, fmt.Errorf("instance %q not found", binding.TargetID)
	}
	if strings.EqualFold(instance.Status, "offline") {
		return nil, fmt.Errorf("instance %q is offline", instance.ID)
	}

	return instance, nil
}

func resolveTargetIP(instance *models.Instance) string {
	if instance.PrivateIP != nil && *instance.PrivateIP != "" {
		return *instance.PrivateIP
	}
	if instance.PublicIP != nil && *instance.PublicIP != "" {
		return *instance.PublicIP
	}
	return ""
}

func (e *ProductionHealthGateEvaluator) buildProbeConfig(hc map[string]any, targetIP string) HealthCheckProbeConfig {
	cfg := e.defaultCfg
	cfg.Host = targetIP

	if protocol, ok := hc["protocol"].(string); ok && protocol != "" {
		cfg.Protocol = strings.ToLower(protocol)
	}
	if cfg.Protocol == "" {
		cfg.Protocol = "http"
	}

	if port, ok := hc["port"].(float64); ok && port > 0 {
		cfg.Port = int(port)
	}
	if cfg.Port == 0 {
		cfg.Port = 8080
	}

	if path, ok := hc["path"].(string); ok && path != "" {
		cfg.Path = path
	}
	if cfg.Path == "" {
		cfg.Path = "/health"
	}

	if timeoutStr, ok := hc["timeout"].(string); ok && timeoutStr != "" {
		if d, err := time.ParseDuration(timeoutStr); err == nil && d > 0 {
			cfg.Timeout = d
		}
	}

	if successThreshold, ok := hc["success_threshold"].(float64); ok && successThreshold > 0 {
		cfg.SuccessThreshold = int(successThreshold)
	}

	if failureThreshold, ok := hc["failure_threshold"].(float64); ok && failureThreshold > 0 {
		cfg.FailureThreshold = int(failureThreshold)
	}

	return cfg
}

type probeResult struct {
	Passed  bool
	Message string
}

func (e *ProductionHealthGateEvaluator) probeService(ctx context.Context, cfg HealthCheckProbeConfig, serviceName string) probeResult {
	successes := 0
	failures := 0

	for i := 0; i < cfg.SuccessThreshold+cfg.FailureThreshold-1; i++ {
		select {
		case <-ctx.Done():
			return probeResult{
				Passed:  false,
				Message: fmt.Sprintf("context cancelled after %d successes, %d failures", successes, failures),
			}
		default:
		}

		ok, msg := e.probeOnce(ctx, cfg)
		if ok {
			successes++
			if successes >= cfg.SuccessThreshold {
				return probeResult{
					Passed:  true,
					Message: fmt.Sprintf("healthy after %d successful probe(s)", successes),
				}
			}
		} else {
			failures++
			if failures >= cfg.FailureThreshold {
				return probeResult{
					Passed:  false,
					Message: fmt.Sprintf("unhealthy after %d failed probe(s): %s", failures, msg),
				}
			}
		}

		if i < cfg.SuccessThreshold+cfg.FailureThreshold-2 {
			select {
			case <-ctx.Done():
				return probeResult{
					Passed:  false,
					Message: fmt.Sprintf("context cancelled during retry after %d successes, %d failures", successes, failures),
				}
			case <-time.After(cfg.RetryInterval):
			}
		}
	}

	if successes >= cfg.SuccessThreshold {
		return probeResult{Passed: true, Message: fmt.Sprintf("healthy after %d successful probe(s)", successes)}
	}
	return probeResult{Passed: false, Message: fmt.Sprintf("unhealthy: %d successes, %d failures", successes, failures)}
}

func (e *ProductionHealthGateEvaluator) probeOnce(ctx context.Context, cfg HealthCheckProbeConfig) (bool, string) {
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))

	switch cfg.Protocol {
	case "http", "https":
		return e.probeHTTP(ctx, cfg, addr)
	case "tcp":
		return e.probeTCP(ctx, addr)
	default:
		return e.probeHTTP(ctx, cfg, addr)
	}
}

func (e *ProductionHealthGateEvaluator) probeHTTP(ctx context.Context, cfg HealthCheckProbeConfig, addr string) (bool, string) {
	scheme := "http"
	if cfg.Protocol == "https" {
		scheme = "https"
	}

	url := fmt.Sprintf("%s://%s%s", scheme, addr, cfg.Path)

	reqCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return false, fmt.Sprintf("failed to build request: %v", err)
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return false, fmt.Sprintf("HTTP probe failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return true, fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
	return false, fmt.Sprintf("HTTP %d (expected 2xx-3xx)", resp.StatusCode)
}

func (e *ProductionHealthGateEvaluator) probeTCP(ctx context.Context, addr string) (bool, string) {
	dialer := net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return false, fmt.Sprintf("TCP probe failed: %v", err)
	}
	conn.Close()
	return true, "TCP connection successful"
}
