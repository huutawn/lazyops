package runtime

import (
	"time"
)

const (
	CommandTypeReconcileRevision       = "reconcile_revision"
	CommandTypePrepareReleaseWorkspace = "prepare_release_workspace"
	CommandTypeEnsureMeshPeer          = "ensure_mesh_peer"
	CommandTypeSyncOverlayRoutes       = "sync_overlay_routes"
	CommandTypeRenderSidecars          = "render_sidecars"
	CommandTypeRenderGatewayConfig     = "render_gateway_config"
	CommandTypeProvisionInternalSvc    = "provision_internal_services"
	CommandTypeStartReleaseCandidate   = "start_release_candidate"
	CommandTypeRunHealthGate           = "run_health_gate"
	CommandTypePromoteRelease          = "promote_release"
	CommandTypeRollbackRelease         = "rollback_release"
	CommandTypeWakeService             = "wake_service"
	CommandTypeSleepService            = "sleep_service"
	CommandTypeReportTopologyState     = "report_topology_state"
	CommandTypeReportTraceSummary      = "report_trace_summary"
	CommandTypeReportMetricRollup      = "report_metric_rollup"
	CommandTypeReportLogBatch          = "report_log_batch"
	CommandTypeGarbageCollectRuntime   = "garbage_collect_runtime"
)

var ValidAgentCommands = map[string]struct{}{
	CommandTypeReconcileRevision:       {},
	CommandTypePrepareReleaseWorkspace: {},
	CommandTypeEnsureMeshPeer:          {},
	CommandTypeSyncOverlayRoutes:       {},
	CommandTypeRenderSidecars:          {},
	CommandTypeRenderGatewayConfig:     {},
	CommandTypeProvisionInternalSvc:    {},
	CommandTypeStartReleaseCandidate:   {},
	CommandTypeRunHealthGate:           {},
	CommandTypePromoteRelease:          {},
	CommandTypeRollbackRelease:         {},
	CommandTypeWakeService:             {},
	CommandTypeSleepService:            {},
	CommandTypeReportTopologyState:     {},
	CommandTypeReportTraceSummary:      {},
	CommandTypeReportMetricRollup:      {},
	CommandTypeReportLogBatch:          {},
	CommandTypeGarbageCollectRuntime:   {},
}

const (
	EventDeploymentStarted        = "deployment.started"
	EventDeploymentBuildFailed    = "deployment.build_failed"
	EventDeploymentCandidateReady = "deployment.candidate_ready"
	EventDeploymentPromoted       = "deployment.promoted"
	EventDeploymentRolledBack     = "deployment.rolled_back"
	EventIncidentCreated          = "incident.created"
	EventTraceRecorded            = "trace.recorded"
	EventTopologyUpdated          = "topology.updated"
	EventMetricRollupIngested     = "metric.rollup_ingested"
)

var ValidOperatorEvents = map[string]struct{}{
	EventDeploymentStarted:        {},
	EventDeploymentBuildFailed:    {},
	EventDeploymentCandidateReady: {},
	EventDeploymentPromoted:       {},
	EventDeploymentRolledBack:     {},
	EventIncidentCreated:          {},
	EventTraceRecorded:            {},
	EventTopologyUpdated:          {},
	EventMetricRollupIngested:     {},
}

type CommandEnvelope struct {
	Type          string         `json:"type"`
	RequestID     string         `json:"request_id"`
	CorrelationID string         `json:"correlation_id"`
	AgentID       string         `json:"agent_id"`
	ProjectID     string         `json:"project_id"`
	Source        string         `json:"source"`
	OccurredAt    time.Time      `json:"occurred_at"`
	Payload       map[string]any `json:"payload"`
}

type OperatorEvent struct {
	Type       string         `json:"type"`
	Payload    any            `json:"payload"`
	OccurredAt time.Time      `json:"occurred_at"`
	Meta       map[string]any `json:"meta,omitempty"`
}

func NewCommandEnvelope(cmdType, requestID, correlationID, agentID, projectID, source string, payload map[string]any) CommandEnvelope {
	return CommandEnvelope{
		Type:          cmdType,
		RequestID:     requestID,
		CorrelationID: correlationID,
		AgentID:       agentID,
		ProjectID:     projectID,
		Source:        source,
		OccurredAt:    time.Now().UTC(),
		Payload:       payload,
	}
}

func NewOperatorEvent(eventType string, payload any, meta map[string]any) OperatorEvent {
	return OperatorEvent{
		Type:       eventType,
		Payload:    payload,
		OccurredAt: time.Now().UTC(),
		Meta:       meta,
	}
}

func IsValidAgentCommand(cmdType string) bool {
	_, ok := ValidAgentCommands[cmdType]
	return ok
}

func IsValidOperatorEvent(eventType string) bool {
	_, ok := ValidOperatorEvents[eventType]
	return ok
}
