package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"lazyops-agent/internal/contracts"
	"lazyops-agent/internal/dispatcher"
	"lazyops-agent/internal/state"
)

type Service struct {
	logger *slog.Logger
	store  *state.Store
	driver Driver
	now    func() time.Time
}

func NewService(logger *slog.Logger, store *state.Store, driver Driver) *Service {
	return &Service{
		logger: logger,
		store:  store,
		driver: driver,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (s *Service) Register(registry *dispatcher.Registry) {
	if registry == nil {
		return
	}
	registry.Register(contracts.CommandPrepareReleaseWorkspace, dispatcher.HandlerFunc(s.handlePrepareReleaseWorkspace))
	registry.Register(contracts.CommandRenderGatewayConfig, dispatcher.HandlerFunc(s.handleRenderGatewayConfig))
	registry.Register(contracts.CommandStartReleaseCandidate, dispatcher.HandlerFunc(s.handleStartReleaseCandidate))
	registry.Register(contracts.CommandRunHealthGate, dispatcher.HandlerFunc(s.handleRunHealthGate))
}

func (s *Service) handlePrepareReleaseWorkspace(ctx context.Context, envelope contracts.CommandEnvelope) dispatcher.Result {
	var payload contracts.PrepareReleaseWorkspacePayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return dispatcher.NonRetryable("invalid_prepare_release_workspace_payload", "command payload could not be decoded", map[string]any{
			"error": err.Error(),
		})
	}

	runtimeCtx, err := ContextFromPreparePayload(payload)
	if err != nil {
		return dispatcher.NonRetryable("invalid_runtime_context", err.Error(), nil)
	}

	prepared, err := s.driver.PrepareReleaseWorkspace(ctx, runtimeCtx)
	if err != nil {
		return dispatcher.Retryable("prepare_release_workspace_failed", err.Error(), map[string]any{
			"revision_id": runtimeCtx.Revision.RevisionID,
			"binding_id":  runtimeCtx.Binding.BindingID,
		})
	}

	if s.store != nil {
		if _, err := s.store.Update(ctx, func(local *state.AgentLocalState) error {
			recordPendingRevision(local, runtimeCtx.Revision.RevisionID, s.now())
			return nil
		}); err != nil {
			return dispatcher.Retryable("persist_runtime_workspace_failed", fmt.Sprintf("workspace prepared but local state update failed: %v", err), map[string]any{
				"revision_id": runtimeCtx.Revision.RevisionID,
				"binding_id":  runtimeCtx.Binding.BindingID,
			})
		}
	}

	if s.logger != nil {
		s.logger.Info("runtime workspace prepared",
			"request_id", envelope.RequestID,
			"correlation_id", envelope.CorrelationID,
			"revision_id", runtimeCtx.Revision.RevisionID,
			"binding_id", runtimeCtx.Binding.BindingID,
			"workspace_root", prepared.Layout.Root,
		)
	}

	return dispatcher.Done("release workspace prepared")
}

func (s *Service) handleStartReleaseCandidate(ctx context.Context, envelope contracts.CommandEnvelope) dispatcher.Result {
	var payload contracts.PrepareReleaseWorkspacePayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return dispatcher.NonRetryable("invalid_start_release_candidate_payload", "command payload could not be decoded", map[string]any{
			"error": err.Error(),
		})
	}

	runtimeCtx, err := ContextFromPreparePayload(payload)
	if err != nil {
		return dispatcher.NonRetryable("invalid_runtime_context", err.Error(), nil)
	}

	candidate, err := s.driver.StartReleaseCandidate(ctx, runtimeCtx)
	if err != nil {
		return dispatcher.Retryable("start_release_candidate_failed", err.Error(), map[string]any{
			"revision_id": runtimeCtx.Revision.RevisionID,
			"binding_id":  runtimeCtx.Binding.BindingID,
		})
	}

	if s.store != nil {
		if _, err := s.store.Update(ctx, func(local *state.AgentLocalState) error {
			recordPendingRevision(local, runtimeCtx.Revision.RevisionID, s.now())
			local.RevisionCache.CandidateRevisionID = runtimeCtx.Revision.RevisionID
			local.RevisionCache.CandidateState = string(candidate.State)
			local.RevisionCache.CandidateWorkspaceRoot = candidate.WorkspaceRoot
			return nil
		}); err != nil {
			return dispatcher.Retryable("persist_candidate_state_failed", fmt.Sprintf("candidate skeleton recorded but local state update failed: %v", err), map[string]any{
				"revision_id": runtimeCtx.Revision.RevisionID,
				"binding_id":  runtimeCtx.Binding.BindingID,
			})
		}
	}

	if s.logger != nil {
		s.logger.Info("runtime candidate skeleton recorded",
			"request_id", envelope.RequestID,
			"correlation_id", envelope.CorrelationID,
			"revision_id", runtimeCtx.Revision.RevisionID,
			"binding_id", runtimeCtx.Binding.BindingID,
			"candidate_state", candidate.State,
		)
	}

	return dispatcher.Done("release candidate skeleton recorded")
}

func (s *Service) handleRenderGatewayConfig(ctx context.Context, envelope contracts.CommandEnvelope) dispatcher.Result {
	var payload contracts.PrepareReleaseWorkspacePayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return dispatcher.NonRetryable("invalid_render_gateway_config_payload", "command payload could not be decoded", map[string]any{
			"error": err.Error(),
		})
	}

	runtimeCtx, err := ContextFromPreparePayload(payload)
	if err != nil {
		return dispatcher.NonRetryable("invalid_runtime_context", err.Error(), nil)
	}

	rendered, err := s.driver.RenderGatewayConfig(ctx, runtimeCtx)
	if err != nil {
		return dispatchOperationError(
			err,
			"render_gateway_config_failed",
			map[string]any{
				"revision_id": runtimeCtx.Revision.RevisionID,
				"binding_id":  runtimeCtx.Binding.BindingID,
			},
		)
	}

	if s.store != nil {
		if _, err := s.store.Update(ctx, func(local *state.AgentLocalState) error {
			recordPendingRevision(local, runtimeCtx.Revision.RevisionID, s.now())
			return nil
		}); err != nil {
			return dispatcher.Retryable("persist_gateway_render_state_failed", fmt.Sprintf("gateway config rendered but local state update failed: %v", err), map[string]any{
				"revision_id": runtimeCtx.Revision.RevisionID,
				"binding_id":  runtimeCtx.Binding.BindingID,
				"version":     rendered.Version,
			})
		}
	}

	if s.logger != nil {
		s.logger.Info("runtime gateway config rendered",
			"request_id", envelope.RequestID,
			"correlation_id", envelope.CorrelationID,
			"revision_id", runtimeCtx.Revision.RevisionID,
			"binding_id", runtimeCtx.Binding.BindingID,
			"gateway_version", rendered.Version,
			"public_urls", len(rendered.PublicURLs),
		)
	}

	return dispatcher.Done(fmt.Sprintf("gateway config rendered and applied as %s", rendered.Version))
}

func (s *Service) handleRunHealthGate(ctx context.Context, envelope contracts.CommandEnvelope) dispatcher.Result {
	var payload contracts.PrepareReleaseWorkspacePayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return dispatcher.NonRetryable("invalid_run_health_gate_payload", "command payload could not be decoded", map[string]any{
			"error": err.Error(),
		})
	}

	runtimeCtx, err := ContextFromPreparePayload(payload)
	if err != nil {
		return dispatcher.NonRetryable("invalid_runtime_context", err.Error(), nil)
	}

	if s.store != nil {
		local, err := s.store.Load(ctx)
		if err != nil {
			return dispatcher.Retryable("load_runtime_state_failed", fmt.Sprintf("could not load local state for health gate: %v", err), map[string]any{
				"revision_id": runtimeCtx.Revision.RevisionID,
				"binding_id":  runtimeCtx.Binding.BindingID,
			})
		}
		runtimeCtx.Rollout.StableRevisionID = local.RevisionCache.StableRevisionID
	}

	report, err := s.driver.RunHealthGate(ctx, runtimeCtx)
	if err != nil {
		return dispatcher.Retryable("run_health_gate_failed", err.Error(), map[string]any{
			"revision_id": runtimeCtx.Revision.RevisionID,
			"binding_id":  runtimeCtx.Binding.BindingID,
		})
	}

	if s.store != nil {
		if _, err := s.store.Update(ctx, func(local *state.AgentLocalState) error {
			recordPendingRevision(local, runtimeCtx.Revision.RevisionID, s.now())
			local.RevisionCache.CandidateRevisionID = runtimeCtx.Revision.RevisionID
			local.RevisionCache.CandidateState = string(report.CandidateState)
			local.RevisionCache.LastHealthGateAt = report.CheckedAt
			local.RevisionCache.LastHealthGateState = string(report.CandidateState)
			local.RevisionCache.LastHealthGateSummary = report.Summary
			local.RevisionCache.LastPolicyAction = string(report.PolicyAction)
			return nil
		}); err != nil {
			return dispatcher.Retryable("persist_health_gate_state_failed", fmt.Sprintf("health gate completed but local state update failed: %v", err), map[string]any{
				"revision_id": runtimeCtx.Revision.RevisionID,
				"binding_id":  runtimeCtx.Binding.BindingID,
			})
		}
	}

	if s.logger != nil {
		s.logger.Info("runtime health gate completed",
			"request_id", envelope.RequestID,
			"correlation_id", envelope.CorrelationID,
			"revision_id", runtimeCtx.Revision.RevisionID,
			"binding_id", runtimeCtx.Binding.BindingID,
			"candidate_state", report.CandidateState,
			"promotable", report.Promotable,
			"policy_action", report.PolicyAction,
		)
	}

	if !report.Promotable {
		details := map[string]any{
			"candidate_state":      report.CandidateState,
			"policy_action":        report.PolicyAction,
			"report_path":          report.ReportPath,
			"rollout_summary_path": report.RolloutSummaryPath,
		}
		if runtimeCtx.Rollout.StableRevisionID != "" {
			details["stable_revision_id"] = runtimeCtx.Rollout.StableRevisionID
		}
		if report.Incident != nil {
			details["incident_kind"] = report.Incident.Kind
			details["incident_severity"] = report.Incident.Severity
		}
		if report.IncidentSuppressed {
			details["incident_suppressed"] = true
		}
		return dispatcher.NonRetryable("health_gate_failed", report.Summary, details)
	}

	return dispatcher.Done(report.Summary)
}

func dispatchOperationError(err error, fallbackCode string, fallbackDetails map[string]any) dispatcher.Result {
	var opErr *OperationError
	if errors.As(err, &opErr) {
		details := make(map[string]any, len(fallbackDetails)+len(opErr.Details))
		for key, value := range fallbackDetails {
			details[key] = value
		}
		for key, value := range opErr.Details {
			details[key] = value
		}
		if opErr.Retryable {
			return dispatcher.Retryable(nonEmpty(opErr.Code, fallbackCode), opErr.Error(), details)
		}
		return dispatcher.NonRetryable(nonEmpty(opErr.Code, fallbackCode), opErr.Error(), details)
	}
	return dispatcher.Retryable(fallbackCode, err.Error(), fallbackDetails)
}

func nonEmpty(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
