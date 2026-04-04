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

type TraceSender interface {
	SendTraceSummary(context.Context, contracts.TraceSummaryPayload) error
}

type Service struct {
	logger           *slog.Logger
	store            *state.Store
	driver           Driver
	traceCollector   *TraceCollector
	traceSender      TraceSender
	logCollector     *LogCollector
	logSender        LogSender
	metricAggregator *MetricAggregator
	metricSender     MetricSender
	nodeMetrics      *NodeMetricsCollector
	topologyReporter *TopologyReporter
	topologySender   TopologySender
	incidentReporter *IncidentReporter
	incidentSender   IncidentSender
	tunnelRelay      *TunnelRelay
	autosleepManager *AutosleepManager
	gatewayHoldMgr   *GatewayHoldManager
	scaleToZeroGuard *ScaleToZeroGuard
	now              func() time.Time
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

func (s *Service) WithTraceCollector(collector *TraceCollector) *Service {
	s.traceCollector = collector
	return s
}

func (s *Service) WithTraceSender(sender TraceSender) *Service {
	s.traceSender = sender
	return s
}

func (s *Service) WithLogCollector(collector *LogCollector) *Service {
	s.logCollector = collector
	return s
}

func (s *Service) WithLogSender(sender LogSender) *Service {
	s.logSender = sender
	return s
}

func (s *Service) WithMetricAggregator(agg *MetricAggregator) *Service {
	s.metricAggregator = agg
	return s
}

func (s *Service) WithMetricSender(sender MetricSender) *Service {
	s.metricSender = sender
	return s
}

func (s *Service) WithNodeMetrics(collector *NodeMetricsCollector) *Service {
	s.nodeMetrics = collector
	return s
}

func (s *Service) WithTopologyReporter(reporter *TopologyReporter) *Service {
	s.topologyReporter = reporter
	return s
}

func (s *Service) WithTopologySender(sender TopologySender) *Service {
	s.topologySender = sender
	return s
}

func (s *Service) WithIncidentReporter(reporter *IncidentReporter) *Service {
	s.incidentReporter = reporter
	return s
}

func (s *Service) WithIncidentSender(sender IncidentSender) *Service {
	s.incidentSender = sender
	return s
}

func (s *Service) WithTunnelRelay(relay *TunnelRelay) *Service {
	s.tunnelRelay = relay
	return s
}

func (s *Service) WithAutosleepManager(mgr *AutosleepManager) *Service {
	s.autosleepManager = mgr
	return s
}

func (s *Service) WithGatewayHoldManager(mgr *GatewayHoldManager) *Service {
	s.gatewayHoldMgr = mgr
	return s
}

func (s *Service) WithScaleToZeroGuard(guard *ScaleToZeroGuard) *Service {
	s.scaleToZeroGuard = guard
	return s
}

func (s *Service) Register(registry *dispatcher.Registry) {
	if registry == nil {
		return
	}
	registry.Register(contracts.CommandPrepareReleaseWorkspace, dispatcher.HandlerFunc(s.handlePrepareReleaseWorkspace))
	registry.Register(contracts.CommandRenderGatewayConfig, dispatcher.HandlerFunc(s.handleRenderGatewayConfig))
	registry.Register(contracts.CommandRenderSidecars, dispatcher.HandlerFunc(s.handleRenderSidecars))
	registry.Register(contracts.CommandStartReleaseCandidate, dispatcher.HandlerFunc(s.handleStartReleaseCandidate))
	registry.Register(contracts.CommandRunHealthGate, dispatcher.HandlerFunc(s.handleRunHealthGate))
	registry.Register(contracts.CommandPromoteRelease, dispatcher.HandlerFunc(s.handlePromoteRelease))
	registry.Register(contracts.CommandRollbackRelease, dispatcher.HandlerFunc(s.handleRollbackRelease))
	registry.Register(contracts.CommandGarbageCollectRuntime, dispatcher.HandlerFunc(s.handleGarbageCollectRuntime))
	registry.Register(contracts.CommandReportTraceSummary, dispatcher.HandlerFunc(s.handleReportTraceSummary))
	registry.Register(contracts.CommandReportLogBatch, dispatcher.HandlerFunc(s.handleReportLogBatch))
	registry.Register(contracts.CommandReportMetricRollup, dispatcher.HandlerFunc(s.handleReportMetricRollup))
	registry.Register(contracts.CommandReportTopologyState, dispatcher.HandlerFunc(s.handleReportTopologyState))
	registry.Register(contracts.CommandSleepService, dispatcher.HandlerFunc(s.handleSleepService))
	registry.Register(contracts.CommandWakeService, dispatcher.HandlerFunc(s.handleWakeService))
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

func (s *Service) handleRenderSidecars(ctx context.Context, envelope contracts.CommandEnvelope) dispatcher.Result {
	var payload contracts.PrepareReleaseWorkspacePayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return dispatcher.NonRetryable("invalid_render_sidecars_payload", "command payload could not be decoded", map[string]any{
			"error": err.Error(),
		})
	}

	runtimeCtx, err := ContextFromPreparePayload(payload)
	if err != nil {
		return dispatcher.NonRetryable("invalid_runtime_context", err.Error(), nil)
	}

	rendered, err := s.driver.RenderSidecars(ctx, runtimeCtx)
	if err != nil {
		return dispatchOperationError(
			err,
			"render_sidecars_failed",
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
			return dispatcher.Retryable("persist_sidecar_render_state_failed", fmt.Sprintf("sidecar config rendered but local state update failed: %v", err), map[string]any{
				"revision_id": runtimeCtx.Revision.RevisionID,
				"binding_id":  runtimeCtx.Binding.BindingID,
				"version":     rendered.Version,
			})
		}
	}

	if s.logger != nil {
		s.logger.Info("runtime sidecar config rendered",
			"request_id", envelope.RequestID,
			"correlation_id", envelope.CorrelationID,
			"revision_id", runtimeCtx.Revision.RevisionID,
			"binding_id", runtimeCtx.Binding.BindingID,
			"sidecar_version", rendered.Version,
			"enabled_services", len(rendered.Services),
		)
	}

	return dispatcher.Done(fmt.Sprintf("sidecar config rendered and applied as %s", rendered.Version))
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
		if err := s.populateRolloutContext(ctx, &runtimeCtx, "health gate"); err != nil {
			return dispatcher.Retryable("load_runtime_state_failed", err.Error(), map[string]any{
				"revision_id": runtimeCtx.Revision.RevisionID,
				"binding_id":  runtimeCtx.Binding.BindingID,
			})
		}
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

func (s *Service) handlePromoteRelease(ctx context.Context, envelope contracts.CommandEnvelope) dispatcher.Result {
	var payload contracts.PrepareReleaseWorkspacePayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return dispatcher.NonRetryable("invalid_promote_release_payload", "command payload could not be decoded", map[string]any{
			"error": err.Error(),
		})
	}

	runtimeCtx, err := ContextFromPreparePayload(payload)
	if err != nil {
		return dispatcher.NonRetryable("invalid_runtime_context", err.Error(), nil)
	}

	if s.store != nil {
		if err := s.populateRolloutContext(ctx, &runtimeCtx, "promotion"); err != nil {
			return dispatcher.Retryable("load_runtime_state_failed", err.Error(), map[string]any{
				"revision_id": runtimeCtx.Revision.RevisionID,
				"binding_id":  runtimeCtx.Binding.BindingID,
			})
		}
	}

	promoted, err := s.driver.PromoteRelease(ctx, runtimeCtx)
	if err != nil {
		return dispatchOperationError(
			err,
			"promote_release_failed",
			map[string]any{
				"revision_id": runtimeCtx.Revision.RevisionID,
				"binding_id":  runtimeCtx.Binding.BindingID,
			},
		)
	}

	if s.store != nil {
		if _, err := s.store.Update(ctx, func(local *state.AgentLocalState) error {
			recordPendingRevision(local, runtimeCtx.Revision.RevisionID, s.now())
			local.RevisionCache.CurrentRevisionID = promoted.RevisionID
			local.RevisionCache.PreviousStableRevisionID = promoted.PreviousStableRevisionID
			local.RevisionCache.StableRevisionID = promoted.RevisionID
			local.RevisionCache.PendingRevisionID = ""
			local.RevisionCache.CandidateRevisionID = ""
			local.RevisionCache.CandidateState = ""
			local.RevisionCache.CandidateWorkspaceRoot = ""
			local.RevisionCache.DrainingRevisionID = promoted.DrainPlan.PreviousRevisionID
			local.RevisionCache.LastPolicyAction = string(HealthGatePolicyPromoteCandidate)
			local.RevisionCache.LastPromotionAt = s.now()
			local.RevisionCache.LastPromotionSummary = promoted.Summary.Summary
			return nil
		}); err != nil {
			return dispatcher.Retryable("persist_promotion_state_failed", fmt.Sprintf("promotion completed but local state update failed: %v", err), map[string]any{
				"revision_id": runtimeCtx.Revision.RevisionID,
				"binding_id":  runtimeCtx.Binding.BindingID,
			})
		}
	}

	if s.logger != nil {
		s.logger.Info("runtime release promoted",
			"request_id", envelope.RequestID,
			"correlation_id", envelope.CorrelationID,
			"revision_id", runtimeCtx.Revision.RevisionID,
			"binding_id", runtimeCtx.Binding.BindingID,
			"previous_stable_revision_id", promoted.PreviousStableRevisionID,
			"rollback_ready", promoted.RollbackReady,
			"gateway_version", promoted.GatewayVersion,
			"sidecar_version", promoted.SidecarVersion,
		)
	}

	return dispatcher.Done(promoted.Summary.Summary)
}

func (s *Service) handleRollbackRelease(ctx context.Context, envelope contracts.CommandEnvelope) dispatcher.Result {
	var payload contracts.PrepareReleaseWorkspacePayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return dispatcher.NonRetryable("invalid_rollback_release_payload", "command payload could not be decoded", map[string]any{
			"error": err.Error(),
		})
	}

	runtimeCtx, err := ContextFromPreparePayload(payload)
	if err != nil {
		return dispatcher.NonRetryable("invalid_runtime_context", err.Error(), nil)
	}

	if s.store != nil {
		if err := s.populateRolloutContext(ctx, &runtimeCtx, "rollback"); err != nil {
			return dispatcher.Retryable("load_runtime_state_failed", err.Error(), map[string]any{
				"revision_id": runtimeCtx.Revision.RevisionID,
				"binding_id":  runtimeCtx.Binding.BindingID,
			})
		}
	}

	rolledBack, err := s.driver.RollbackRelease(ctx, runtimeCtx)
	if err != nil {
		return dispatchOperationError(
			err,
			"rollback_release_failed",
			map[string]any{
				"revision_id": runtimeCtx.Revision.RevisionID,
				"binding_id":  runtimeCtx.Binding.BindingID,
			},
		)
	}

	if s.store != nil {
		if _, err := s.store.Update(ctx, func(local *state.AgentLocalState) error {
			local.RevisionCache.CurrentRevisionID = rolledBack.RestoredRevisionID
			local.RevisionCache.StableRevisionID = rolledBack.RestoredRevisionID
			local.RevisionCache.PreviousStableRevisionID = ""
			local.RevisionCache.PendingRevisionID = ""
			local.RevisionCache.CandidateRevisionID = ""
			local.RevisionCache.CandidateState = ""
			local.RevisionCache.CandidateWorkspaceRoot = ""
			local.RevisionCache.DrainingRevisionID = rolledBack.FailedRevisionID
			local.RevisionCache.LastPolicyAction = string(HealthGatePolicyRollbackRelease)
			local.RevisionCache.LastRollbackAt = s.now()
			local.RevisionCache.LastRollbackFromRevision = rolledBack.FailedRevisionID
			local.RevisionCache.LastRollbackToRevision = rolledBack.RestoredRevisionID
			local.RevisionCache.LastRollbackSummary = rolledBack.Summary.Summary
			local.RevisionCache.UpdatedAt = s.now()
			return nil
		}); err != nil {
			return dispatcher.Retryable("persist_rollback_state_failed", fmt.Sprintf("rollback completed but local state update failed: %v", err), map[string]any{
				"revision_id": runtimeCtx.Revision.RevisionID,
				"binding_id":  runtimeCtx.Binding.BindingID,
			})
		}
	}

	if s.logger != nil {
		s.logger.Warn("runtime release rolled back",
			"request_id", envelope.RequestID,
			"correlation_id", envelope.CorrelationID,
			"revision_id", rolledBack.FailedRevisionID,
			"binding_id", runtimeCtx.Binding.BindingID,
			"restored_revision_id", rolledBack.RestoredRevisionID,
			"incident_kind", nonNilIncidentKind(rolledBack.Incident),
			"incident_severity", nonNilIncidentSeverity(rolledBack.Incident),
		)
	}

	return dispatcher.Done(rolledBack.Summary.Summary)
}

func (s *Service) handleGarbageCollectRuntime(ctx context.Context, envelope contracts.CommandEnvelope) dispatcher.Result {
	var payload contracts.PrepareReleaseWorkspacePayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return dispatcher.NonRetryable("invalid_garbage_collect_runtime_payload", "command payload could not be decoded", map[string]any{
			"error": err.Error(),
		})
	}

	runtimeCtx, err := ContextFromPreparePayload(payload)
	if err != nil {
		return dispatcher.NonRetryable("invalid_runtime_context", err.Error(), nil)
	}

	if s.store != nil {
		if err := s.populateRolloutContext(ctx, &runtimeCtx, "runtime garbage collection"); err != nil {
			return dispatcher.Retryable("load_runtime_state_failed", err.Error(), map[string]any{
				"revision_id": runtimeCtx.Revision.RevisionID,
				"binding_id":  runtimeCtx.Binding.BindingID,
			})
		}
	}

	collected, err := s.driver.GarbageCollectRuntime(ctx, runtimeCtx)
	if err != nil {
		return dispatchOperationError(
			err,
			"garbage_collect_runtime_failed",
			map[string]any{
				"revision_id": runtimeCtx.Revision.RevisionID,
				"binding_id":  runtimeCtx.Binding.BindingID,
			},
		)
	}

	if s.store != nil {
		if _, err := s.store.Update(ctx, func(local *state.AgentLocalState) error {
			local.RevisionCache.LastRuntimeGCAt = collected.CollectedAt
			local.RevisionCache.LastRuntimeGCSummary = collected.Summary
			local.RevisionCache.UpdatedAt = s.now()
			return nil
		}); err != nil {
			return dispatcher.Retryable("persist_runtime_gc_state_failed", fmt.Sprintf("runtime gc completed but local state update failed: %v", err), map[string]any{
				"revision_id": runtimeCtx.Revision.RevisionID,
				"binding_id":  runtimeCtx.Binding.BindingID,
			})
		}
	}

	if s.logger != nil {
		s.logger.Info("runtime garbage collection completed",
			"request_id", envelope.RequestID,
			"correlation_id", envelope.CorrelationID,
			"binding_id", runtimeCtx.Binding.BindingID,
			"removed_revisions", len(collected.RemovedRevisionRoots),
			"removed_gateway_versions", len(collected.RemovedGatewayVersions),
			"removed_sidecar_versions", len(collected.RemovedSidecarVersions),
		)
	}

	return dispatcher.Done(collected.Summary)
}

func (s *Service) populateRolloutContext(ctx context.Context, runtimeCtx *RuntimeContext, operation string) error {
	if s.store == nil || runtimeCtx == nil {
		return nil
	}

	local, err := s.store.Load(ctx)
	if err != nil {
		return fmt.Errorf("could not load local state for %s: %w", operation, err)
	}
	runtimeCtx.Rollout.CurrentRevisionID = local.RevisionCache.CurrentRevisionID
	runtimeCtx.Rollout.StableRevisionID = local.RevisionCache.StableRevisionID
	runtimeCtx.Rollout.PreviousStableRevisionID = local.RevisionCache.PreviousStableRevisionID
	runtimeCtx.Rollout.PendingRevisionID = local.RevisionCache.PendingRevisionID
	runtimeCtx.Rollout.CandidateRevisionID = local.RevisionCache.CandidateRevisionID
	runtimeCtx.Rollout.DrainingRevisionID = local.RevisionCache.DrainingRevisionID
	return nil
}

func nonNilIncidentKind(incident *contracts.IncidentPayload) string {
	if incident == nil {
		return ""
	}
	return incident.Kind
}

func nonNilIncidentSeverity(incident *contracts.IncidentPayload) contracts.Severity {
	if incident == nil {
		return ""
	}
	return incident.Severity
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

func (s *Service) handleReportTraceSummary(ctx context.Context, envelope contracts.CommandEnvelope) dispatcher.Result {
	var payload ReportTraceSummaryPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return dispatcher.NonRetryable("invalid_report_trace_summary_payload", "command payload could not be decoded", map[string]any{
			"error": err.Error(),
		})
	}

	if s.traceCollector == nil {
		return dispatcher.NonRetryable("trace_collector_not_configured", "trace collector is not initialized", nil)
	}

	payload.TraceSender = s.traceSender

	reported, err := s.traceCollector.HandleReportTraceSummary(ctx, s.logger, payload)
	if err != nil {
		return dispatcher.Retryable("report_trace_summary_failed", err.Error(), map[string]any{
			"project_id": payload.ProjectID,
			"binding_id": payload.BindingID,
		})
	}

	return dispatcher.Done(fmt.Sprintf("trace summary report completed, %d windows reported", reported))
}

func (s *Service) handleReportLogBatch(ctx context.Context, envelope contracts.CommandEnvelope) dispatcher.Result {
	var payload ReportLogBatchPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return dispatcher.NonRetryable("invalid_report_log_batch_payload", "command payload could not be decoded", map[string]any{
			"error": err.Error(),
		})
	}

	if s.logCollector == nil {
		return dispatcher.NonRetryable("log_collector_not_configured", "log collector is not initialized", nil)
	}

	payload.LogSender = s.logSender

	reported, err := s.logCollector.HandleReportLogBatch(ctx, s.logger, payload)
	if err != nil {
		return dispatcher.Retryable("report_log_batch_failed", err.Error(), map[string]any{
			"project_id": payload.ProjectID,
			"binding_id": payload.BindingID,
		})
	}

	return dispatcher.Done(fmt.Sprintf("log batch report completed, %d batches reported", reported))
}

func (s *Service) handleReportMetricRollup(ctx context.Context, envelope contracts.CommandEnvelope) dispatcher.Result {
	var payload ReportMetricRollupPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return dispatcher.NonRetryable("invalid_report_metric_rollup_payload", "command payload could not be decoded", map[string]any{
			"error": err.Error(),
		})
	}

	if s.metricAggregator == nil {
		return dispatcher.NonRetryable("metric_aggregator_not_configured", "metric aggregator is not initialized", nil)
	}

	payload.MetricSender = s.metricSender

	reported, err := s.metricAggregator.HandleReportMetricRollup(ctx, s.logger, payload)
	if err != nil {
		return dispatcher.Retryable("report_metric_rollup_failed", err.Error(), map[string]any{
			"project_id": payload.ProjectID,
			"binding_id": payload.BindingID,
		})
	}

	return dispatcher.Done(fmt.Sprintf("metric rollup report completed, %d rollups reported", reported))
}

func (s *Service) handleReportTopologyState(ctx context.Context, envelope contracts.CommandEnvelope) dispatcher.Result {
	var payload ReportTopologyStatePayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return dispatcher.NonRetryable("invalid_report_topology_payload", "command payload could not be decoded", map[string]any{
			"error": err.Error(),
		})
	}

	if s.topologyReporter == nil {
		return dispatcher.NonRetryable("topology_reporter_not_configured", "topology reporter is not initialized", nil)
	}

	payload.TopologySender = s.topologySender

	if err := s.topologyReporter.HandleReportTopologyState(ctx, s.logger, payload); err != nil {
		return dispatcher.Retryable("report_topology_state_failed", err.Error(), map[string]any{
			"project_id": payload.ProjectID,
			"binding_id": payload.BindingID,
		})
	}

	nodes, edges := s.topologyReporter.Stats()
	return dispatcher.Done(fmt.Sprintf("topology state reported, %d nodes, %d edges", nodes, edges))
}

func (s *Service) handleReportIncidents(ctx context.Context, envelope contracts.CommandEnvelope) dispatcher.Result {
	var payload ReportIncidentPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return dispatcher.NonRetryable("invalid_report_incident_payload", "command payload could not be decoded", map[string]any{
			"error": err.Error(),
		})
	}

	if s.incidentReporter == nil {
		return dispatcher.NonRetryable("incident_reporter_not_configured", "incident reporter is not initialized", nil)
	}

	payload.IncidentSender = s.incidentSender

	reported, err := s.incidentReporter.HandleReportIncidents(ctx, s.logger, payload)
	if err != nil {
		return dispatcher.Retryable("report_incidents_failed", err.Error(), map[string]any{
			"project_id": payload.ProjectID,
			"binding_id": payload.BindingID,
		})
	}

	return dispatcher.Done(fmt.Sprintf("incident report completed, %d incidents reported", reported))
}

func (s *Service) handleReportTunnelState(ctx context.Context, envelope contracts.CommandEnvelope) dispatcher.Result {
	var payload ReportTunnelStatePayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return dispatcher.NonRetryable("invalid_report_tunnel_payload", "command payload could not be decoded", map[string]any{
			"error": err.Error(),
		})
	}

	if s.tunnelRelay == nil {
		return dispatcher.NonRetryable("tunnel_relay_not_configured", "tunnel relay is not initialized", nil)
	}

	expired, err := s.tunnelRelay.HandleReportTunnelState(ctx, s.logger, payload)
	if err != nil {
		return dispatcher.Retryable("report_tunnel_state_failed", err.Error(), map[string]any{
			"project_id": payload.ProjectID,
			"binding_id": payload.BindingID,
		})
	}

	total, active, expiredCount := s.tunnelRelay.Stats()
	return dispatcher.Done(fmt.Sprintf("tunnel state reported, %d total, %d active, %d expired", total, active, expired+expiredCount))
}

type SleepServicePayload struct {
	ProjectID   string                      `json:"project_id"`
	BindingID   string                      `json:"binding_id"`
	RevisionID  string                      `json:"revision_id"`
	RuntimeMode contracts.RuntimeMode       `json:"runtime_mode"`
	ServiceName string                      `json:"service_name"`
	Policy      contracts.ScaleToZeroPolicy `json:"scale_to_zero_policy"`
}

type WakeServicePayload struct {
	ProjectID   string                `json:"project_id"`
	BindingID   string                `json:"binding_id"`
	RevisionID  string                `json:"revision_id"`
	RuntimeMode contracts.RuntimeMode `json:"runtime_mode"`
	ServiceName string                `json:"service_name"`
}

func (s *Service) handleSleepService(ctx context.Context, envelope contracts.CommandEnvelope) dispatcher.Result {
	var payload SleepServicePayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return dispatcher.NonRetryable("invalid_sleep_service_payload", "command payload could not be decoded", map[string]any{
			"error": err.Error(),
		})
	}

	if s.autosleepManager == nil {
		return dispatcher.NonRetryable("autosleep_manager_not_configured", "autosleep manager is not initialized", nil)
	}

	if s.scaleToZeroGuard != nil {
		if err := s.scaleToZeroGuard.ValidateSleepPolicy(payload.ServiceName, payload.Policy, payload.RuntimeMode); err != nil {
			return dispatcher.NonRetryable("scale_to_zero_policy_violation", err.Error(), map[string]any{
				"service_name": payload.ServiceName,
				"runtime_mode": string(payload.RuntimeMode),
			})
		}

		if !s.scaleToZeroGuard.CanSleep(payload.ServiceName, payload.Policy, payload.RuntimeMode) {
			return dispatcher.NonRetryable("service_not_eligible_for_sleep", "service does not meet sleep eligibility criteria", map[string]any{
				"service_name": payload.ServiceName,
			})
		}

		state, err := s.scaleToZeroGuard.SleepService(payload.ServiceName, payload.RevisionID, payload.RuntimeMode)
		if err != nil {
			return dispatcher.NonRetryable("sleep_service_failed", err.Error(), map[string]any{
				"service_name": payload.ServiceName,
			})
		}

		if s.gatewayHoldMgr != nil {
			_ = s.gatewayHoldMgr.ResumeRequests(payload.ServiceName)
		}

		return dispatcher.Done(fmt.Sprintf("service %s put to sleep at %s", payload.ServiceName, state.SleepingAt.Format(time.RFC3339)))
	}

	state, err := s.autosleepManager.SleepService(payload.ServiceName, payload.RevisionID)
	if err != nil {
		return dispatcher.NonRetryable("sleep_service_failed", err.Error(), map[string]any{
			"service_name": payload.ServiceName,
		})
	}

	if s.gatewayHoldMgr != nil {
		_ = s.gatewayHoldMgr.ResumeRequests(payload.ServiceName)
	}

	return dispatcher.Done(fmt.Sprintf("service %s put to sleep at %s", payload.ServiceName, state.SleepingAt.Format(time.RFC3339)))
}

func (s *Service) handleWakeService(ctx context.Context, envelope contracts.CommandEnvelope) dispatcher.Result {
	var payload WakeServicePayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return dispatcher.NonRetryable("invalid_wake_service_payload", "command payload could not be decoded", map[string]any{
			"error": err.Error(),
		})
	}

	if s.autosleepManager == nil {
		return dispatcher.NonRetryable("autosleep_manager_not_configured", "autosleep manager is not initialized", nil)
	}

	if s.scaleToZeroGuard != nil {
		if timedOut, attempts := s.scaleToZeroGuard.CheckWakeTimeout(payload.ServiceName); timedOut {
			return dispatcher.NonRetryable("wake_timeout_exceeded", fmt.Sprintf("service %s exceeded wake timeout after %d attempts", payload.ServiceName, attempts), map[string]any{
				"service_name": payload.ServiceName,
				"attempts":     attempts,
			})
		}

		if s.scaleToZeroGuard.CheckColdStartTimeout(payload.ServiceName) {
			return dispatcher.NonRetryable("cold_start_timeout_exceeded", fmt.Sprintf("service %s exceeded cold start attempts", payload.ServiceName), map[string]any{
				"service_name": payload.ServiceName,
			})
		}

		state, err := s.scaleToZeroGuard.WakeService(payload.ServiceName)
		if err != nil {
			return dispatcher.Retryable("wake_service_failed", err.Error(), map[string]any{
				"service_name": payload.ServiceName,
			})
		}

		s.scaleToZeroGuard.MarkActive(payload.ServiceName)

		return dispatcher.Done(fmt.Sprintf("service %s woke at %s", payload.ServiceName, state.LastActiveAt.Format(time.RFC3339)))
	}

	state, err := s.autosleepManager.WakeService(payload.ServiceName)
	if err != nil {
		return dispatcher.Retryable("wake_service_failed", err.Error(), map[string]any{
			"service_name": payload.ServiceName,
		})
	}

	s.autosleepManager.MarkActive(payload.ServiceName)

	return dispatcher.Done(fmt.Sprintf("service %s woke at %s", payload.ServiceName, state.LastActiveAt.Format(time.RFC3339)))
}
