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
)

func (d *FilesystemDriver) RunHealthGate(ctx context.Context, runtimeCtx RuntimeContext) (HealthGateResult, error) {
	layout := workspaceLayout(d.root, runtimeCtx)

	candidate, err := loadCandidateRecord(layout)
	if err != nil {
		return HealthGateResult{}, fmt.Errorf("load candidate manifest: %w", err)
	}

	report := HealthGateResult{
		RevisionID: runtimeCtx.Revision.RevisionID,
		CheckedAt:  d.now(),
		Services:   make([]ServiceHealthResult, 0, len(runtimeCtx.Services)),
	}

	passedServices := 0
	failingServices := make([]string, 0)
	for _, service := range runtimeCtx.Services {
		serviceResult := runServiceHealthCheck(ctx, service, report.CheckedAt)
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

	address := net.JoinHostPort("127.0.0.1", fmt.Sprintf("%d", service.HealthCheck.Port))
	result := ServiceHealthResult{
		ServiceName: service.Name,
		Protocol:    protocol,
		Address:     address,
		Path:        service.HealthCheck.Path,
		CheckedAt:   checkedAt,
	}
	if service.HealthCheck.Port <= 0 {
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
		failureThreshold = 1
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
		passed, statusCode, latencyMS, message := probeServiceOnce(ctx, protocol, address, service.HealthCheck.Path, timeout)
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
