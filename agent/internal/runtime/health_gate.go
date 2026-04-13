package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"lazyops-agent/internal/contracts"
)

const (
	defaultHealthCheckTimeout = 2 * time.Second
	defaultHealthRetryDelay   = 100 * time.Millisecond
	defaultStartupGracePeriod = 45 * time.Second
	defaultDBStartupGrace     = 75 * time.Second
	defaultFailureThreshold   = 3
	appProbeTimeout           = 300 * time.Millisecond
)

var healthProbeOnce = probeServiceOnce

func (d *FilesystemDriver) RunHealthGate(ctx context.Context, runtimeCtx RuntimeContext) (HealthGateResult, error) {
	layout := workspaceLayout(d.root, runtimeCtx)
	runtimeCtx = d.hydrateRuntimeContextFromWorkspace(layout, runtimeCtx)

	candidate, err := loadCandidateRecord(layout)
	if err != nil {
		return HealthGateResult{}, fmt.Errorf("load candidate manifest: %w", err)
	}

	// Wait for startup grace period to allow services to initialize
	// This is critical for services like Postgres that need time to boot
	// Use the maximum startup grace period across all services, or the default if none specify it
	startupGracePeriod := time.Duration(0)
	hasExplicitGracePeriod := false
	for _, service := range runtimeCtx.Services {
		if strings.TrimSpace(service.HealthCheck.StartupGracePeriod) != "" {
			if parsed, err := time.ParseDuration(service.HealthCheck.StartupGracePeriod); err == nil {
				hasExplicitGracePeriod = true
				if parsed > startupGracePeriod {
					startupGracePeriod = parsed
				}
			}
		}
	}
	if !hasExplicitGracePeriod {
		startupGracePeriod = defaultStartupGracePeriod
		if hasInternalPostgresDependency(runtimeCtx.Services) {
			startupGracePeriod = defaultDBStartupGrace
		}
	}

	if d.logger != nil {
		d.logger.Info("health_gate_startup_delay", "duration", startupGracePeriod.String())
	}

	select {
	case <-ctx.Done():
		return HealthGateResult{}, fmt.Errorf("startup grace period cancelled: %w", ctx.Err())
	case <-time.After(startupGracePeriod):
	}

	if d.logger != nil {
		d.logger.Info("health_gate_probing_started")
	}

	report := HealthGateResult{
		RevisionID: runtimeCtx.Revision.RevisionID,
		CheckedAt:  d.now(),
		Services:   make([]ServiceHealthResult, 0, len(runtimeCtx.Services)),
	}

	passedServices := 0
	failingServices := make([]string, 0)
	for _, service := range runtimeCtx.Services {
		// Skip internal services (lazyops-internal-*) - they're managed infrastructure
		// and already verified during provision_internal_services
		if strings.HasPrefix(service.Name, "lazyops-internal-") || service.Name == "lazyops-internal-service" {
			if d.logger != nil {
				d.logger.Info("health_gate_skipping_internal_service", "service", service.Name)
			}
			report.Services = append(report.Services, ServiceHealthResult{
				ServiceName: service.Name,
				Passed:      true,
				Message:     "internal service managed by lazyops - skipped",
			})
			passedServices++
			continue
		}

		if skip, reason := shouldSkipServiceHealthCheck(runtimeCtx, service); skip {
			protocol := strings.ToLower(strings.TrimSpace(service.HealthCheck.Protocol))
			if protocol == "" {
				protocol = "http"
			}
			address := ""
			port := effectiveRuntimePort(service)
			if port > 0 {
				address = net.JoinHostPort("127.0.0.1", fmt.Sprintf("%d", port))
			}
			if d.logger != nil {
				d.logger.Info("health_gate_skipping_service",
					"service", service.Name,
					"reason", reason,
				)
			}
			report.Services = append(report.Services, ServiceHealthResult{
				ServiceName: service.Name,
				Protocol:    protocol,
				Address:     address,
				Path:        service.HealthCheck.Path,
				Passed:      true,
				Message:     reason,
				CheckedAt:   report.CheckedAt,
			})
			passedServices++
			continue
		}

		serviceResult := runServiceHealthCheck(ctx, service, report.CheckedAt)
		serviceResult = softenFailedAppHealthCheck(runtimeCtx, service, serviceResult)
		report.Services = append(report.Services, serviceResult)
		if serviceResult.Passed {
			passedServices++
			continue
		}
		failingServices = append(failingServices, service.Name)
	}

	if len(failingServices) == 0 {
		if err := moveCandidateToPromotable(&candidate, report.CheckedAt); err != nil {
			return HealthGateResult{}, err
		}
		report.Promotable = true
		report.CandidateState = candidate.State
		report.PolicyAction = HealthGatePolicyPromoteCandidate
		report.Summary = fmt.Sprintf("health gate passed for %d/%d services; candidate is promotable", passedServices, len(runtimeCtx.Services))
		if d.logger != nil {
			d.logger.Info("health_gate_passed", "passed", passedServices, "total", len(runtimeCtx.Services))
		}
	} else {
		if err := moveCandidateToFailed(&candidate, report.CheckedAt); err != nil {
			return HealthGateResult{}, err
		}
		report.CandidateState = candidate.State
		report.PolicyAction = HealthGatePolicyStopRollout
		if strings.TrimSpace(runtimeCtx.Rollout.StableRevisionID) != "" {
			report.PolicyAction = HealthGatePolicyRollbackRelease
		}
		report.Summary = fmt.Sprintf("health gate failed for %d/%d services: %s", len(failingServices), len(runtimeCtx.Services), strings.Join(failingServices, ", "))
		if d.logger != nil {
			d.logger.Error("health_gate_failed", "failing", len(failingServices), "total", len(runtimeCtx.Services), "services", strings.Join(failingServices, ", "))
		}

		incident := contracts.IncidentPayload{
			ProjectID:  runtimeCtx.Project.ProjectID,
			RevisionID: runtimeCtx.Revision.RevisionID,
			Severity:   contracts.SeverityWarning,
			Kind:       "deployment_health_gate_failed",
			Summary:    report.Summary,
			OccurredAt: report.CheckedAt,
			Details: map[string]any{
				"candidate_state":    candidate.State,
				"policy_action":      report.PolicyAction,
				"stable_revision_id": runtimeCtx.Rollout.StableRevisionID,
				"failing_services":   failingServices,
			},
		}
		incidentKey := healthIncidentKey(report.PolicyAction, failingServices)
		if incidentKey == candidate.LastIncidentKey {
			report.IncidentSuppressed = true
		} else {
			report.Incident = &incident
			candidate.LatestIncident = &incident
			candidate.LastIncidentKey = incidentKey
			candidate.LastIncidentAt = report.CheckedAt
		}
	}

	rolloutSummary := RolloutSummary{
		RevisionID:        runtimeCtx.Revision.RevisionID,
		CandidateState:    candidate.State,
		PolicyAction:      report.PolicyAction,
		Summary:           report.Summary,
		HealthyServices:   passedServices,
		UnhealthyServices: len(runtimeCtx.Services) - passedServices,
		CheckedAt:         report.CheckedAt,
	}

	candidate.HealthGate = &CandidateHealthSnapshot{
		CheckedAt:          report.CheckedAt,
		CandidateState:     candidate.State,
		Promotable:         report.Promotable,
		PolicyAction:       report.PolicyAction,
		Summary:            report.Summary,
		Services:           report.Services,
		Incident:           report.Incident,
		IncidentSuppressed: report.IncidentSuppressed,
	}
	candidate.RolloutSummary = &rolloutSummary

	report.ReportPath = healthGateReportPath(layout)
	report.RolloutSummaryPath = rolloutSummaryPath(layout)
	if err := writeJSON(report.ReportPath, report); err != nil {
		return HealthGateResult{}, err
	}
	if err := writeJSON(report.RolloutSummaryPath, rolloutSummary); err != nil {
		return HealthGateResult{}, err
	}
	if err := saveCandidateRecord(candidate); err != nil {
		return HealthGateResult{}, err
	}

	if d.logger != nil {
		logMethod := d.logger.Info
		if !report.Promotable && !report.IncidentSuppressed {
			logMethod = d.logger.Warn
		}
		logMethod("health gate completed",
			"revision_id", runtimeCtx.Revision.RevisionID,
			"candidate_state", candidate.State,
			"promotable", report.Promotable,
			"policy_action", report.PolicyAction,
		)
	}

	return report, nil
}

func moveCandidateToPromotable(record *CandidateRecord, at time.Time) error {
	switch record.State {
	case CandidateStateStarting:
		if err := transitionCandidateState(record, CandidateStateHealthy, "health checks passed", at); err != nil {
			return err
		}
		if err := transitionCandidateState(record, CandidateStatePromotable, "candidate is promotable", at); err != nil {
			return err
		}
	case CandidateStateUnhealthy:
		if err := transitionCandidateState(record, CandidateStateHealthy, "health checks recovered", at); err != nil {
			return err
		}
		if err := transitionCandidateState(record, CandidateStatePromotable, "candidate is promotable", at); err != nil {
			return err
		}
	case CandidateStateHealthy:
		if err := transitionCandidateState(record, CandidateStatePromotable, "candidate is promotable", at); err != nil {
			return err
		}
	case CandidateStatePromotable:
		return nil
	case CandidateStateFailed:
		return fmt.Errorf("candidate %q is already failed and must be restarted before another health gate", record.RevisionID)
	default:
		return fmt.Errorf("candidate %q is not ready for health gate success transition from %q", record.RevisionID, record.State)
	}
	return nil
}

func moveCandidateToFailed(record *CandidateRecord, at time.Time) error {
	switch record.State {
	case CandidateStateStarting, CandidateStateHealthy, CandidateStatePromotable:
		if err := transitionCandidateState(record, CandidateStateUnhealthy, "health checks failed", at); err != nil {
			return err
		}
		if err := transitionCandidateState(record, CandidateStateFailed, "candidate failed health gate", at); err != nil {
			return err
		}
	case CandidateStateUnhealthy:
		if err := transitionCandidateState(record, CandidateStateFailed, "candidate failed health gate", at); err != nil {
			return err
		}
	case CandidateStateFailed:
		return nil
	default:
		return fmt.Errorf("candidate %q is not ready for health gate failure transition from %q", record.RevisionID, record.State)
	}
	return nil
}

func runServiceHealthCheck(ctx context.Context, service ServiceRuntimeContext, checkedAt time.Time) ServiceHealthResult {
	protocol := strings.ToLower(strings.TrimSpace(service.HealthCheck.Protocol))
	if protocol == "" {
		protocol = "http"
	}

	ports := healthCheckPortCandidates(service, protocol)
	if len(ports) == 0 {
		return ServiceHealthResult{
			ServiceName: service.Name,
			Protocol:    protocol,
			Path:        service.HealthCheck.Path,
			CheckedAt:   checkedAt,
			Failures:    1,
			Message:     "healthcheck port must be greater than zero",
		}
	}

	runtimePort := service.RuntimePort
	declaredPort := declaredHealthcheckPort(service)

	var lastResult ServiceHealthResult
	for _, port := range ports {
		lastResult = runServiceHealthCheckOnPort(ctx, service, checkedAt, protocol, port)
		if lastResult.Passed {
			if port != declaredPort && declaredPort > 0 {
				lastResult.Message = fmt.Sprintf("%s; fallback port %d selected", lastResult.Message, port)
			}
			return lastResult
		}
	}
	if len(ports) > 1 {
		lastResult.Message = fmt.Sprintf("%s; tried ports [%s]", strings.TrimSpace(lastResult.Message), joinPorts(ports))
	}
	if runtimePort > 0 && declaredPort > 0 && runtimePort != declaredPort {
		lastResult.Message = fmt.Sprintf("%s; declared healthcheck port %d differs from runtime port %d", strings.TrimSpace(lastResult.Message), declaredPort, runtimePort)
	}
	return lastResult
}

func runServiceHealthCheckOnPort(
	ctx context.Context,
	service ServiceRuntimeContext,
	checkedAt time.Time,
	protocol string,
	port int,
) ServiceHealthResult {
	address := net.JoinHostPort("127.0.0.1", fmt.Sprintf("%d", port))
	result := ServiceHealthResult{
		ServiceName: service.Name,
		Protocol:    protocol,
		Address:     address,
		Path:        service.HealthCheck.Path,
		CheckedAt:   checkedAt,
	}
	if port <= 0 {
		result.Failures = 1
		result.Message = "healthcheck port must be greater than zero"
		return result
	}

	successThreshold := service.HealthCheck.SuccessThreshold
	if successThreshold <= 0 {
		successThreshold = 1
	}
	failureThreshold := service.HealthCheck.FailureThreshold
	if failureThreshold <= 0 {
		// Use a small default retry window for startup jitter and warmup latency.
		failureThreshold = defaultFailureThreshold
	}

	timeout := defaultHealthCheckTimeout
	if strings.TrimSpace(service.HealthCheck.Timeout) != "" {
		if parsed, err := time.ParseDuration(service.HealthCheck.Timeout); err == nil && parsed > 0 {
			timeout = parsed
		}
	}

	maxAttempts := successThreshold + failureThreshold
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	consecutiveSuccesses := 0
	consecutiveFailures := 0
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		result.Attempts = attempt
		passed, statusCode, latencyMS, message := healthProbeOnce(ctx, protocol, address, service.HealthCheck.Path, timeout)
		result.StatusCode = statusCode
		result.LatencyMS = latencyMS
		result.Message = message

		if passed {
			consecutiveSuccesses++
			consecutiveFailures = 0
			result.Successes = consecutiveSuccesses
			result.Failures = 0
			if consecutiveSuccesses >= successThreshold {
				result.Passed = true
				if strings.TrimSpace(result.Message) == "" {
					result.Message = "health check passed"
				}
				return result
			}
		} else {
			consecutiveFailures++
			consecutiveSuccesses = 0
			result.Successes = 0
			result.Failures = consecutiveFailures
			if consecutiveFailures >= failureThreshold {
				return result
			}
		}

		if attempt < maxAttempts {
			select {
			case <-ctx.Done():
				result.Message = ctx.Err().Error()
				return result
			case <-time.After(retryDelay(timeout)):
			}
		}
	}

	result.Message = "health gate exhausted attempts without reaching success threshold"
	return result
}

func healthCheckPortCandidates(service ServiceRuntimeContext, protocol string) []int {
	ports := make([]int, 0, 3)
	seen := make(map[int]struct{}, 3)
	addPort := func(port int) {
		if port <= 0 {
			return
		}
		if _, exists := seen[port]; exists {
			return
		}
		seen[port] = struct{}{}
		ports = append(ports, port)
	}

	addPort(service.RuntimePort)
	addPort(service.HealthCheck.Port)

	if strings.EqualFold(strings.TrimSpace(service.Name), "app") {
		switch strings.ToLower(strings.TrimSpace(protocol)) {
		case "http", "https":
			addPort(3000)
			addPort(5000)
		}
	}

	return ports
}

func joinPorts(ports []int) string {
	if len(ports) == 0 {
		return ""
	}
	parts := make([]string, 0, len(ports))
	for _, port := range ports {
		parts = append(parts, fmt.Sprintf("%d", port))
	}
	return strings.Join(parts, ",")
}

func probeServiceOnce(ctx context.Context, protocol, address, path string, timeout time.Duration) (bool, int, float64, string) {
	switch protocol {
	case "http", "https":
		if strings.TrimSpace(path) == "" {
			path = "/"
		}
		startedAt := time.Now()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s://%s%s", protocol, address, path), nil)
		if err != nil {
			return false, 0, 0, err.Error()
		}
		client := &http.Client{Timeout: timeout}
		resp, err := client.Do(req)
		if err != nil {
			return false, 0, float64(time.Since(startedAt).Milliseconds()), err.Error()
		}
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)
		latencyMS := float64(time.Since(startedAt).Milliseconds())
		if resp.StatusCode >= 200 && resp.StatusCode < 400 {
			return true, resp.StatusCode, latencyMS, fmt.Sprintf("http health check passed with status %d", resp.StatusCode)
		}
		return false, resp.StatusCode, latencyMS, fmt.Sprintf("http health check returned status %d", resp.StatusCode)
	case "tcp":
		startedAt := time.Now()
		dialer := &net.Dialer{Timeout: timeout}
		conn, err := dialer.DialContext(ctx, "tcp", address)
		if err != nil {
			return false, 0, float64(time.Since(startedAt).Milliseconds()), err.Error()
		}
		_ = conn.Close()
		return true, 0, float64(time.Since(startedAt).Milliseconds()), "tcp health check passed"
	default:
		return false, 0, 0, fmt.Sprintf("unsupported healthcheck protocol %q", protocol)
	}
}

func retryDelay(timeout time.Duration) time.Duration {
	delay := timeout / 4
	if delay <= 0 {
		delay = defaultHealthRetryDelay
	}
	if delay > defaultHealthRetryDelay {
		return defaultHealthRetryDelay
	}
	return delay
}

func healthIncidentKey(action HealthGatePolicyAction, failingServices []string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s", action, strings.Join(failingServices, ","))))
	return hex.EncodeToString(sum[:8])
}

func shouldSkipServiceHealthCheck(runtimeCtx RuntimeContext, service ServiceRuntimeContext) (bool, string) {
	// In one-click flows, "app" may be a placeholder service before users
	// actually ship a runnable app process. Skip health gate for that case.
	if strings.ToLower(strings.TrimSpace(service.Name)) != "app" {
		return false, ""
	}
	if !isOneClickAutogenRevision(runtimeCtx.Revision) {
		return false, ""
	}
	port := effectiveRuntimePort(service)
	if port <= 0 {
		return true, "app has no healthcheck port configured; skipped"
	}
	address := net.JoinHostPort("127.0.0.1", fmt.Sprintf("%d", port))
	conn, err := net.DialTimeout("tcp", address, appProbeTimeout)
	if err != nil {
		return true, fmt.Sprintf("app listener not detected on %s; skipped", address)
	}
	_ = conn.Close()
	return false, ""
}

func softenFailedAppHealthCheck(runtimeCtx RuntimeContext, service ServiceRuntimeContext, result ServiceHealthResult) ServiceHealthResult {
	if result.Passed {
		return result
	}
	if strings.ToLower(strings.TrimSpace(service.Name)) != "app" {
		return result
	}
	if !isOneClickAutogenRevision(runtimeCtx.Revision) {
		return result
	}

	protocol := strings.ToLower(strings.TrimSpace(result.Protocol))
	if protocol == "" {
		protocol = strings.ToLower(strings.TrimSpace(service.HealthCheck.Protocol))
	}
	if protocol != "http" && protocol != "https" {
		return result
	}
	port := effectiveRuntimePort(service)
	if port <= 0 {
		return result
	}

	address := net.JoinHostPort("127.0.0.1", fmt.Sprintf("%d", port))
	conn, err := net.DialTimeout("tcp", address, appProbeTimeout)
	if err != nil {
		return result
	}
	_ = conn.Close()

	result.Passed = true
	result.Failures = 0
	if result.Successes <= 0 {
		result.Successes = 1
	}
	if strings.TrimSpace(result.Message) == "" {
		result.Message = fmt.Sprintf("app healthcheck soft-passed for one-click autogen; tcp listener reachable at %s", address)
	} else {
		result.Message = fmt.Sprintf("%s; soft-passed for one-click autogen because tcp listener is reachable at %s", result.Message, address)
	}
	return result
}

func isOneClickAutogenRevision(revision contracts.DesiredRevisionPayload) bool {
	if !strings.EqualFold(strings.TrimSpace(revision.TriggerKind), "one_click_deploy") {
		return false
	}
	commit := strings.TrimSpace(revision.CommitSHA)
	if strings.HasPrefix(commit, "autogen-") {
		return true
	}
	artifactRef := strings.TrimSpace(revision.ArtifactRef)
	return strings.Contains(artifactRef, "/autogen-")
}

func hasInternalPostgresDependency(services []ServiceRuntimeContext) bool {
	for _, service := range services {
		name := strings.ToLower(strings.TrimSpace(service.Name))
		if strings.HasPrefix(name, "lazyops-internal-") {
			continue
		}
		for _, dep := range service.Dependencies {
			if strings.EqualFold(strings.TrimSpace(dep.TargetService), "lazyops-internal-postgres") {
				return true
			}
		}
	}
	return false
}
