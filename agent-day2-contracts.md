# Agent Day 2 Contract Foundation

## Summary

Day 2 locks the first agent transport and payload contracts in both documentation and code. The main outcome of this day is that the repo now has a concrete Go package for agent contracts under `agent/internal/contracts`, plus this document to explain the conventions that downstream teams should follow.

Day 2 conclusions:

- The agent contract surface is now explicit enough for backend and agent work to proceed in parallel.
- The outbound control session is locked to `GET /ws/agents/control`.
- Command names are locked to the master-plan set and mapped to stable handler ownership areas.
- The minimum payload family for enrollment, auth, heartbeat, capability reporting, rollout, topology, tracing, incidents, and metric rollups is now represented in code.

## Source Documents

- `guide/agent-guide.md`
- `guide/lazyops-implementation-master-plan.md`
- `guide/lazyops-master-des.md`
- `guide/project-rules.md`
- `agent-day1-audit.md`
- `agent-task.md`

## Locked Package Surface

The Day 2 package surface is:

- `agent/internal/contracts/doc.go`
- `agent/internal/contracts/types.go`
- `agent/internal/contracts/control.go`
- `agent/internal/contracts/agent.go`
- `agent/internal/contracts/commands.go`
- `agent/internal/contracts/revision.go`
- `agent/internal/contracts/telemetry.go`

The package is intentionally stdlib-only so it can remain easy to reuse and easy to test.

## Locked Transport Conventions

### Control session path

- Control websocket path is locked to `GET /ws/agents/control`.

### Command envelope conventions

- `type`: the semantic command name. For backend-to-agent commands, it must use the locked command constants such as `reconcile_revision` or `render_sidecars`.
- `request_id`: unique per command attempt. Retries should create a new command attempt and therefore a new `request_id`.
- `correlation_id`: shared across a rollout, request trace, or incident chain so logs, traces, and telemetry can be stitched together.
- `occurred_at`: UTC timestamp for when the sender emitted the envelope.
- `source`: identifies the originator, currently locked to backend/agent/gateway/sidecar producers.
- `payload`: command-specific JSON payload. It stays opaque at the transport shell and is decoded by the command handler.

### Ack and error conventions

- Ack envelopes use `type: command.ack`.
- Error envelopes use `type: command.error`.
- Ack/error messages must echo `request_id`, `correlation_id`, `agent_id`, and `command_type`.
- Errors must explicitly mark `retryable: true|false`.

## Locked Payload Families

### Enrollment and session auth

- `EnrollAgentRequest`
- `EnrollAgentResponse`
- `SessionAuthPayload`
- `AgentHandshakePayload`

These lock the week-1 enrollment/auth family around bootstrap exchange, agent token auth, machine identity, and capability advertisement.

### Heartbeat and capability reporting

- `HeartbeatPayload`
- `CapabilityReportPayload`
- `ControlChannelCapability`
- `GatewayCapability`
- `SidecarCapability`
- `MeshCapability`
- `TelemetryCapability`
- `NodeCapability`
- `PerformanceTargets`

These are the minimum contracts needed for online state, capability snapshots, and future diff-based capability updates.

### Revision and binding payloads

- `DesiredRevisionPayload`
- `DeploymentBindingPayload`
- `PlacementPolicy`
- `DomainPolicy`
- `CompatibilityPolicy`
- `MagicDomainPolicy`
- `ScaleToZeroPolicy`
- `ServicePayload`
- `HealthCheckPayload`
- `DependencyBindingPayload`
- `PlacementAssignment`

These lock the minimum runtime and policy payloads the agent needs without making the agent the source of truth for deploy authority.

### Telemetry payloads

- `TraceSummaryPayload`
- `IncidentPayload`
- `MetricRollupPayload`
- `TopologyPayload`

These contracts lock summarized observability instead of requiring raw full-fidelity capture as the default transport model.

## Locked Minimum Command Set

The locked command set is:

- `reconcile_revision`
- `prepare_release_workspace`
- `ensure_mesh_peer`
- `sync_overlay_routes`
- `render_sidecars`
- `render_gateway_config`
- `start_release_candidate`
- `run_health_gate`
- `promote_release`
- `rollback_release`
- `wake_service`
- `sleep_service`
- `report_topology_state`
- `report_trace_summary`
- `report_metric_rollup`
- `garbage_collect_runtime`

## Locked Command-to-Handler Ownership

| Command | Locked module | Locked handler key |
| --- | --- | --- |
| `reconcile_revision` | `instance/deploy` | `deploy.reconcile_revision` |
| `prepare_release_workspace` | `instance/runtime` | `runtime.prepare_release_workspace` |
| `ensure_mesh_peer` | `instance/mesh` | `mesh.ensure_mesh_peer` |
| `sync_overlay_routes` | `instance/mesh` | `mesh.sync_overlay_routes` |
| `render_sidecars` | `instance/sidecar` | `sidecar.render_sidecars` |
| `render_gateway_config` | `instance/gateway` | `gateway.render_gateway_config` |
| `start_release_candidate` | `instance/deploy` | `deploy.start_release_candidate` |
| `run_health_gate` | `instance/health` | `health.run_health_gate` |
| `promote_release` | `instance/deploy` | `deploy.promote_release` |
| `rollback_release` | `instance/rollback` | `rollback.rollback_release` |
| `wake_service` | `instance/autosleep` | `autosleep.wake_service` |
| `sleep_service` | `instance/autosleep` | `autosleep.sleep_service` |
| `report_topology_state` | `telemetry/topology` | `topology.report_topology_state` |
| `report_trace_summary` | `telemetry/tracing` | `tracing.report_trace_summary` |
| `report_metric_rollup` | `telemetry/metrics` | `metrics.report_metric_rollup` |
| `garbage_collect_runtime` | `instance/runtime` | `runtime.garbage_collect_runtime` |

## Day 2 Exit Decisions

- Future work should extend these contracts instead of inventing parallel payload shapes.
- Backend mocks and agent runtime code should both import or mirror the locked shapes from this contract package.
- Day 3 should now build process bootstrap, config, redaction, local state, and mock control-plane client code on top of these locked contracts.
