package contracts

type CommandType string

const (
	CommandReconcileRevision       CommandType = "reconcile_revision"
	CommandPrepareReleaseWorkspace CommandType = "prepare_release_workspace"
	CommandEnsureMeshPeer          CommandType = "ensure_mesh_peer"
	CommandSyncOverlayRoutes       CommandType = "sync_overlay_routes"
	CommandRenderSidecars          CommandType = "render_sidecars"
	CommandRenderGatewayConfig     CommandType = "render_gateway_config"
	CommandStartReleaseCandidate   CommandType = "start_release_candidate"
	CommandRunHealthGate           CommandType = "run_health_gate"
	CommandPromoteRelease          CommandType = "promote_release"
	CommandRollbackRelease         CommandType = "rollback_release"
	CommandWakeService             CommandType = "wake_service"
	CommandSleepService            CommandType = "sleep_service"
	CommandReportTopologyState     CommandType = "report_topology_state"
	CommandReportTraceSummary      CommandType = "report_trace_summary"
	CommandReportMetricRollup      CommandType = "report_metric_rollup"
	CommandGarbageCollectRuntime   CommandType = "garbage_collect_runtime"
)

var MinimumCommandSet = []CommandType{
	CommandReconcileRevision,
	CommandPrepareReleaseWorkspace,
	CommandEnsureMeshPeer,
	CommandSyncOverlayRoutes,
	CommandRenderSidecars,
	CommandRenderGatewayConfig,
	CommandStartReleaseCandidate,
	CommandRunHealthGate,
	CommandPromoteRelease,
	CommandRollbackRelease,
	CommandWakeService,
	CommandSleepService,
	CommandReportTopologyState,
	CommandReportTraceSummary,
	CommandReportMetricRollup,
	CommandGarbageCollectRuntime,
}

type CommandHandlerSpec struct {
	Command     CommandType `json:"command"`
	Module      string      `json:"module"`
	HandlerKey  string      `json:"handler_key"`
	Description string      `json:"description"`
}

var CommandHandlerBindings = map[CommandType]CommandHandlerSpec{
	CommandReconcileRevision: {
		Command:     CommandReconcileRevision,
		Module:      "instance/deploy",
		HandlerKey:  "deploy.reconcile_revision",
		Description: "Entry point for revision reconciliation and downstream rollout work.",
	},
	CommandPrepareReleaseWorkspace: {
		Command:     CommandPrepareReleaseWorkspace,
		Module:      "instance/runtime",
		HandlerKey:  "runtime.prepare_release_workspace",
		Description: "Prepare local revision workspace, assets, and rendered config directories.",
	},
	CommandEnsureMeshPeer: {
		Command:     CommandEnsureMeshPeer,
		Module:      "instance/mesh",
		HandlerKey:  "mesh.ensure_mesh_peer",
		Description: "Ensure the desired peer exists and matches provider state.",
	},
	CommandSyncOverlayRoutes: {
		Command:     CommandSyncOverlayRoutes,
		Module:      "instance/mesh",
		HandlerKey:  "mesh.sync_overlay_routes",
		Description: "Sync mesh routes from placement and peer health state.",
	},
	CommandRenderSidecars: {
		Command:     CommandRenderSidecars,
		Module:      "instance/sidecar",
		HandlerKey:  "sidecar.render_sidecars",
		Description: "Render sidecar configuration from compatibility and dependency policies.",
	},
	CommandRenderGatewayConfig: {
		Command:     CommandRenderGatewayConfig,
		Module:      "instance/gateway",
		HandlerKey:  "gateway.render_gateway_config",
		Description: "Render Caddy gateway configuration for public ingress and traffic shift.",
	},
	CommandStartReleaseCandidate: {
		Command:     CommandStartReleaseCandidate,
		Module:      "instance/deploy",
		HandlerKey:  "deploy.start_release_candidate",
		Description: "Start the candidate workload for a desired revision.",
	},
	CommandRunHealthGate: {
		Command:     CommandRunHealthGate,
		Module:      "instance/health",
		HandlerKey:  "health.run_health_gate",
		Description: "Run rollout health checks and report promotable state.",
	},
	CommandPromoteRelease: {
		Command:     CommandPromoteRelease,
		Module:      "instance/deploy",
		HandlerKey:  "deploy.promote_release",
		Description: "Promote a healthy candidate and shift traffic.",
	},
	CommandRollbackRelease: {
		Command:     CommandRollbackRelease,
		Module:      "instance/rollback",
		HandlerKey:  "rollback.rollback_release",
		Description: "Return traffic to the last known stable revision.",
	},
	CommandWakeService: {
		Command:     CommandWakeService,
		Module:      "instance/autosleep",
		HandlerKey:  "autosleep.wake_service",
		Description: "Wake a sleeping service workload so held requests can resume.",
	},
	CommandSleepService: {
		Command:     CommandSleepService,
		Module:      "instance/autosleep",
		HandlerKey:  "autosleep.sleep_service",
		Description: "Sleep a workload that has opted into scale-to-zero.",
	},
	CommandReportTopologyState: {
		Command:     CommandReportTopologyState,
		Module:      "telemetry/topology",
		HandlerKey:  "topology.report_topology_state",
		Description: "Report service placement, node health, gateway health, and mesh health.",
	},
	CommandReportTraceSummary: {
		Command:     CommandReportTraceSummary,
		Module:      "telemetry/tracing",
		HandlerKey:  "tracing.report_trace_summary",
		Description: "Report summarized request trace hops and latency signals.",
	},
	CommandReportMetricRollup: {
		Command:     CommandReportMetricRollup,
		Module:      "telemetry/metrics",
		HandlerKey:  "metrics.report_metric_rollup",
		Description: "Report edge-downsampled metric windows to the backend.",
	},
	CommandGarbageCollectRuntime: {
		Command:     CommandGarbageCollectRuntime,
		Module:      "instance/runtime",
		HandlerKey:  "runtime.garbage_collect_runtime",
		Description: "Clean stale workspaces, revisions, and rendered runtime artifacts.",
	},
}
