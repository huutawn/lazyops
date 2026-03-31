# Agent 30-Day Task Tracker

## Source of Truth

The following source documents are mandatory for every task in this file:

- `guide/agent-guide.md`
- `guide/lazyops-implementation-master-plan.md`
- `guide/lazyops-master-des.md`

If any task, implementation note, or status update conflicts with the three documents above, the task must be corrected to match the guides. The guides must not be changed just to justify the task.

Current baseline:

- `agent/cmd/server/main.go` is almost empty.
- This track is written as a live 30-day tracker to move the agent from an initial scaffold to a runtime bridge that matches the spec.
- The required execution order is to lock contracts early, build shared runtime foundations, complete `instance_agent`, extend to `distributed-mesh`, add sidecar and telemetry capabilities, and then add `node_agent` telemetry for `distributed-k3s`.

## Mandatory Compliance Rules

All tasks below must comply with the following rules by default:

- The agent is the runtime bridge between the control plane and the user's infrastructure, and it must follow a `zero-inbound` model; the agent only connects outbound to LazyOps.
- `Long-lived SSH private keys` must not be stored for normal operations; bootstrap must prefer a `short-lived bootstrap token`, a one-time install command, or one-time SSH bootstrap that does not retain credentials long term.
- Plaintext `PAT`, `agent token`, `GitHub token`, `mesh key`, bootstrap token, managed secret, or any other secret must never be logged.
- `instance_agent` may only be used for `standalone` and `distributed-mesh`.
- `node_agent` may only be used for `distributed-k3s`, must operate in a telemetry/protection role, and must never become a scheduler for user workloads.
- In `distributed-k3s`, LazyOps must not bypass K3s to `docker run`, `docker stop`, or directly place user workloads.
- `lazyops.yaml` is the project's deploy contract, but it may contain only `logical intent`; it must not contain SSH credentials, private keys, raw kubeconfig, server passwords, or direct deploy authority.
- Mapping from repo to real target must go through backend-managed `DeploymentBinding`; the repo only stores `target_ref`, and only the backend resolves the real target.
- The compatibility sidecar is a core feature, not an optional helper.
- The mandatory sidecar precedence order is `env injection -> managed credential injection -> localhost rescue`.
- When services communicate across multiple servers, private cross-node traffic may only go through `WireGuard` or `Tailscale`.
- Public ingress must go through `Caddy Gateway` by default; magic domain support must prefer `sslip.io` and fall back to `nip.io`.
- `Zero-downtime rollout` is the default deployment policy.
- `Global rollback` is mandatory when a promoted revision becomes unhealthy.
- `Scale-to-zero` is opt-in only; no service may be forced to sleep if the policy is not enabled.
- Every request entering the public gateway must receive an `X-Correlation-ID`.
- Metrics sent to the backend must be `edge-downsampled rollups`; the agent must aggregate before sending and must not push raw high-volume samples into long-term storage.
- The agent must publish contracts early so backend, CLI, and frontend can move in parallel; if the backend is not ready yet, the agent must use mocks that match the locked schemas and must not invent fields outside the spec.
- The v1 goal of the agent is to support all three runtime modes: `standalone`, `distributed-mesh`, and `distributed-k3s`, without pushing the whole system into a `K3s-first` direction.

## Status Update Rules

- Every task must end with exactly one status: `(pending)`, `(doing)`, or `(done)`.
- All newly created tasks start as `(pending)`.
- When work begins on a task, only that task should be changed to `(doing)`.
- A task may only be changed to `(done)` after the related implementation or documentation is complete, the relevant guides have been checked again, and the change has been self-verified.
- No more than one task should be in `(doing)` status within the same day section.
- If a task depends on a previous unfinished task, the later task must stay `(pending)`.
- Do not delete recorded tasks, and do not reorder tasks just to make progress look cleaner.
- If work spills into the next day, keep the original task in its original day and add a follow-up task on the next day; do not rewrite the status history of the earlier day.

Status update examples:

- `- Lock the agent handshake schema. (doing)`
- `- Lock the agent handshake schema. (done)`
- `- Implement the heartbeat manager and capability reporter. (pending)`

## Public Contracts To Lock In Week 1

The following contracts must be locked in week 1 before implementation scales up:

- Lock the `Enroll Agent` schema: input includes `bootstrap token`, `machine info`, and `capabilities`; output includes `agent token` and `agent id`; validation must reject expired tokens, reused tokens, and ownership mismatch.
- Lock the `GET /ws/agents/control` endpoint for the outbound control session from agent to backend.
- Lock the shared `command envelope` using one consistent shape for internal commands and events.
- Lock the `desired revision payload` so the agent can reconcile revisions without depending on backend implementation details.
- Lock the `deployment binding payload` so the agent understands target mode, policy, and valid reconcile scope.
- Lock the `trace summary payload`, `incident payload`, `metric rollup payload`, and `topology payload`.
- Lock the heartbeat payload, capability summary payload, agent session auth contract, and command ack/error envelope.

Command names must remain exactly as defined in the master plan:

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

## Day 1 - Rule Lock and Baseline Audit

Goal: lock the compliance checklist, baseline audit, and target module map for the agent.

- Audit the current code under `agent/`, documenting what already exists and what is still missing compared with `agent-guide.md`. (done)
- Create a compliance checklist that maps every agent rule, runtime boundary, and security boundary from the three source documents. (done)
- Lock the high-level module map for `shared modules`, `instance agent modules`, and `node agent modules` so the roadmap uses one consistent structure. (done)
- Lock the overall agent state machine including `bootstrap`, `connected`, `reconciling`, `degraded`, `sleeping`, `reporting`, and `disconnected`. (done)

## Day 2 - Contract Foundation

Goal: lock the contracts that allow backend and agent to move in parallel from the start.

- Define package contracts for handshake, session auth, heartbeat, capability reporting, and the command ack/error envelope. (done)
- Lock the JSON schema for the shared `command envelope` and the conventions for `request_id`, `correlation_id`, `occurred_at`, and `source`. (done)
- Define the `desired revision payload`, `deployment binding payload`, `trace summary payload`, `incident payload`, `metric rollup payload`, and `topology payload`. (done)
- Lock the minimum command list and the command-to-handler mapping so later implementation does not drift on names. (done)

## Day 3 - Bootstrap Skeleton and Local Runtime Shell

Goal: build a runnable agent skeleton before the full feature set exists.

- Scaffold `config`, `lifecycle bootstrap`, `logger`, `signal handling`, and `graceful shutdown` for the agent process. (pending)
- Add `logger redaction` to mask tokens, secrets, mesh keys, bootstrap tokens, and sensitive credentials. (pending)
- Create a `local state store` for agent metadata, enrollment state, revision cache, and capability snapshots. (pending)
- Create a `mock control-plane client` using the locked contracts so the agent team can develop in parallel with the backend. (pending)

## Day 4 - Enrollment Flow

Goal: complete the exchange from bootstrap token to agent token in a secure way.

- Implement the `Enroll Agent` flow to send `bootstrap token`, `machine info`, and `capabilities`, and receive `agent token` and `agent id`. (pending)
- Add secure persistence for `agent token` and enrollment metadata inside the local state store. (pending)
- Validate and handle `expired token`, `reused token`, `target mismatch`, and `partial enrollment failure` scenarios. (pending)
- Ensure the bootstrap token is invalidated immediately after successful enrollment and is never logged in plaintext. (pending)

## Day 5 - Outbound Control Connection

Goal: bring the agent onto an outbound session that matches the `zero-inbound` posture.

- Implement the outbound connection to `GET /ws/agents/control` using `agent token` auth. (pending)
- Add reconnect strategy, exponential backoff, jitter, and session resume behavior if the backend allows it. (pending)
- Lock the read/write pump, ping/pong, keepalive, and disconnect detection behavior for the control channel. (pending)
- Ensure this session model is the main channel for deployments and does not expose a persistent inbound control port. (pending)

## Day 6 - Heartbeat and Capability Reporter

Goal: allow the backend to know that the agent is online and what it can do.

- Implement a heartbeat scheduler that sends health and session liveness on a fixed interval. (pending)
- Implement capability reporting for runtime mode, network capability, gateway capability, sidecar support, and node telemetry support. (pending)
- Add local health evaluation to distinguish `online`, `degraded`, `offline`, and `busy` before reporting. (pending)
- Define a capability diff-update contract to avoid resending the full payload when it is not necessary. (pending)

## Day 7 - Command Dispatcher and Handler Registry

Goal: establish one central command processing layer for all runtime behavior.

- Implement the command dispatcher using the shared envelope and routing by `type`. (pending)
- Create a handler registry for the minimum command set locked in the master plan. (pending)
- Add request correlation, structured logging by `request_id` and `correlation_id`, and consistent error mapping. (pending)
- Implement ack, nack, retryable error, and non-retryable error envelopes for the control session. (pending)

## Day 8 - Runtime Driver Abstraction

Goal: build a runtime abstraction that supports `standalone` and `distributed-mesh` while preserving the public `service` contract.

- Define the `runtime driver` interface for prepare workspace, start candidate, health gate, promote, rollback, sleep, wake, and garbage collection. (pending)
- Implement the `prepare_release_workspace` baseline and local workspace layout for artifacts, config, sidecar assets, and gateway assets. (pending)
- Create a runtime context containing project metadata, service metadata, binding policy, and revision payload. (pending)
- Ensure this abstraction does not leak `container` as a public contract and works only in terms of `service` and `revision`. (pending)

## Day 9 - Release Workspace and Candidate Bootstrap

Goal: allow a candidate release to be hydrated and started at a baseline level.

- Implement artifact/image fetch abstraction and local caching for revision assets. (pending)
- Hydrate the `release workspace` from the desired revision, including service manifests, sidecar config, and gateway plan. (pending)
- Implement the `start_release_candidate` skeleton in the runtime driver and record candidate lifecycle state. (pending)
- Add cleanup for incomplete workspace state so deployments do not leave broken leftovers behind. (pending)

## Day 10 - Health Gate and Candidate State Machine

Goal: promote a candidate only after health is valid.

- Implement `run_health_gate` for HTTP/TCP health checks with clear timeout, retry, and success thresholds. (pending)
- Define the candidate state machine including `prepared`, `starting`, `healthy`, `unhealthy`, `promotable`, and `failed`. (pending)
- Add policy handling for health gate failure so the backend can roll back or stop rollout early. (pending)
- Ensure health signals feed incidents and rollout summary without causing log spam. (pending)

## Day 11 - Gateway Manager

Goal: manage `Caddy Gateway` according to the public ingress spec.

- Implement the `gateway manager` to render and apply `Caddy` config from the desired revision. (pending)
- Implement the `render_gateway_config` command and store a versioned gateway plan in the local workspace. (pending)
- Add apply, reload, validate, and rollback hooks for gateway config changes. (pending)
- Ensure the public URL and TLS pipeline start with `sslip.io`, support `nip.io` fallback, and do not expose internal service ports publicly by default. (pending)

## Day 12 - Sidecar Manager Baseline

Goal: make the sidecar a first-class runtime component as required by the spec.

- Implement the `sidecar manager` lifecycle for creating, reconciling, restarting, and removing sidecars by revision. (pending)
- Implement the `render_sidecars` command to generate sidecar config from dependency bindings and compatibility policy. (pending)
- Create hooks to inject the sidecar into the service runtime context without forcing users to hand-write networking glue. (pending)
- Store sidecar metadata in cache so future rescue, tracing, and metrics features can build on it without changing the contract. (pending)

## Day 13 - Promote Release

Goal: complete zero-downtime promotion for the baseline `instance_agent`.

- Implement `promote_release` to shift traffic to the candidate only after the health gate passes. (pending)
- Add traffic draining for the previous revision and cleanup policy after successful promotion. (pending)
- Ensure rollout is zero-downtime by default and still has a rollback path if the candidate becomes unhealthy after promotion. (pending)
- Record rollout events, required latency signals, and deployment summary so the backend can show deployment history. (pending)

## Day 14 - Rollback and Runtime Garbage Collection

Goal: complete the resilience loop for the standalone baseline.

- Implement `rollback_release` to return traffic to the previous stable revision. (pending)
- Emit an incident when a promoted revision becomes unhealthy and send all required signals to the backend. (pending)
- Implement `garbage_collect_runtime` to safely clean old revisions, stale workspaces, and stale config. (pending)
- Ensure rollback never accidentally deletes the stable revision that still needs to remain available as fallback. (pending)

## Day 15 - Mesh Manager Core

Goal: establish the foundation for `distributed-mesh` without requiring Kubernetes.

- Implement the `mesh manager` core with peer model, route cache, membership state, and health summary. (pending)
- Create a `service metadata cache` to map service, alias, target service, and placement through binding logic. (pending)
- Standardize the mesh peer lifecycle model with `planned`, `joining`, `active`, `degraded`, `leaving`, and `removed`. (pending)
- Ensure `standalone` mode fully disables the mesh manager, while `distributed-mesh` enables it. (pending)

## Day 16 - WireGuard Provider Baseline

Goal: provide the default mesh provider required by the spec.

- Implement the baseline provider for `WireGuard` to create, update, and delete mesh peers deterministically. (pending)
- Add key handling, config rendering, and peer cleanup without logging plaintext mesh secrets. (pending)
- Implement the `ensure_mesh_peer` command to ensure desired peer state matches actual state. (pending)
- Record mesh health signals and capability signals needed for backend and telemetry flows. (pending)

## Day 17 - Overlay Route Sync and Tailscale Slot

Goal: complete route orchestration for private cross-node traffic.

- Implement `sync_overlay_routes` to synchronize routes across placement, mesh peer state, and the service metadata cache. (pending)
- Add mesh link health reporting so degraded or unusable routes are clearly visible. (pending)
- Create a reserved adapter slot for `Tailscale` without changing the locked runtime contract. (pending)
- Ensure private service traffic only goes through verified overlay paths and never falls back to public routes. (pending)

## Day 18 - Mesh-Aware Placement and Dependency Resolution Hooks

Goal: allow the agent to know whether a dependency is local or remote and route it correctly.

- Inject `deployment binding payload` and placement data into the runtime context of the agent. (pending)
- Add mesh-aware dependency resolution hooks for local services, remote services, and not-yet-ready services. (pending)
- Define cache invalidation rules for placement changes after rollout or peer health changes. (pending)
- Ensure resolve logic serves sidecar and gateway needs without taking source-of-truth ownership away from the backend. (pending)

## Day 19 - Sidecar Env Injection

Goal: begin implementing the first precedence level of the compatibility sidecar.

- Implement `env injection` for apps that respect env-based dependency configuration. (pending)
- Generate env values from dependency binding resolution without leaking secrets to logs. (pending)
- Add validation that rejects env contracts missing required keys or referencing missing target services. (pending)
- Ensure `env injection` is always the first layer in the precedence chain before trying anything else. (pending)

## Day 20 - Managed Credential Injection

Goal: support the second precedence level while preserving the security rules.

- Implement `managed credential injection` for internal dependencies that need system-managed credentials. (pending)
- Add secret masking, secure in-memory handling, and audit points for the credential path. (pending)
- Write a `no-plaintext logging audit` checklist for the full sidecar, mesh, gateway, and enrollment flow. (pending)
- Ensure that when managed credentials succeed, the system does not continue into `localhost rescue`. (pending)

## Day 21 - Localhost Rescue

Goal: complete the final precedence level for apps that hard-code `localhost`.

- Implement `localhost rescue` for `http` and `tcp` through the sidecar inside the same network namespace. (pending)
- Support forwarding to a local target or a remote mesh target depending on actual placement. (pending)
- Add health-aware fallback when a mesh route is unhealthy so service-down and network-down states can be distinguished. (pending)
- Ensure the precedence chain always remains `env injection -> managed credential injection -> localhost rescue`. (pending)

## Day 22 - Correlation and Trace Summary

Goal: allow the agent to contribute observability that matches the spec without requiring full-fidelity tracing.

- Implement correlation propagation across gateway, sidecar, and runtime command flow based on `X-Correlation-ID`. (pending)
- Measure hop latency, local latency signals, and edge timing in the sidecar and agent. (pending)
- Implement `report_trace_summary` to send summarized trace hop data to the backend. (pending)
- Define sampling and reporting windows that keep trace summaries useful for debugging without making them too heavy. (pending)

## Day 23 - Log Collector

Goal: collect logs in a way that supports hot-path performance and incident debugging.

- Implement `log collector` support for service runtime, gateway output, and sidecar output that matters. (pending)
- Use byte matching first and avoid heavy regex on the hot path, in line with the observability rules. (pending)
- Add cooldown windows to prevent alert storms and duplicate excerpt forwarding. (pending)
- Forward only relevant log excerpts to the control plane instead of streaming full raw log volume when it is unnecessary. (pending)

## Day 24 - Metric Rollup and Node Metrics

Goal: complete edge downsampling before metrics are sent to the backend.

- Implement edge rollups for `p95`, `max`, `min`, `avg`, and `count` across CPU, RAM, latency, and other required metrics. (pending)
- Implement the `report_metric_rollup` command using the payload locked in week 1. (pending)
- Add `node metrics collector` support for `instance_agent` and to prepare the foundation for `node_agent`. (pending)
- Ensure the agent sends aggregate windows only and never sends raw high-volume samples into long-term backend storage. (pending)

## Day 25 - Topology, Incidents, and Tunnel Relay

Goal: make topology and incidents first-class runtime signals.

- Implement `report_topology_state` for service placement, mesh link health, gateway health, and node health. (pending)
- Implement `incident reporter` support to send runtime incidents by revision, severity, kind, and summary. (pending)
- Implement the baseline `tunnel relay` for debug tunnels and trace signal relay when needed. (pending)
- Ensure topology signals fit the UI topology contract and do not introduce fields outside the locked contract. (pending)

## Day 26 - Autosleep Manager

Goal: build the foundation for `scale-to-zero` in `standalone` and `distributed-mesh`.

- Implement `autosleep manager` logic based on the service idle window policy. (pending)
- Implement the `sleep_service` and `wake_service` commands in the runtime driver. (pending)
- Add a gateway hold-and-resume mechanism for the first request after the service is sleeping. (pending)
- Ensure service metadata remains available so the wake-up flow can resolve the correct target and revision. (pending)

## Day 27 - Scale-to-Zero Guard Rails

Goal: turn autosleep into a safe feature with the right boundaries.

- Implement policy guards so only opt-in services may use `scale-to-zero`. (pending)
- Add wake timeout handling, cold-start timeout behavior, and failure signaling for held requests. (pending)
- Implement metadata restore flow after wake-up so the service continues serving correctly after sleep. (pending)
- Ensure `distributed-k3s` remains staged scope only and does not become first-class for `scale-to-zero` in this 30-day baseline. (pending)

## Day 28 - Node Agent Telemetry Mode

Goal: start `node_agent` for `distributed-k3s` in the correct role without drifting into scheduling.

- Implement `k3s environment detector` and mode switching between `instance_agent` and `node_agent`. (pending)
- Create baseline bootstrap assets for `node_agent` in a `DaemonSet` model. (pending)
- Implement `container log tailer` and `node metrics collector` for `node_agent`. (pending)
- State clearly in code and docs that `node_agent` is telemetry/protection only and does not directly place user workloads. (pending)

## Day 29 - Cluster Topology and Incident Reporting

Goal: complete cluster-facing observability for `distributed-k3s`.

- Implement `pod topology reporter` and cluster node topology summary for `node_agent`. (pending)
- Implement `cluster incident reporter` for signals such as unhealthy nodes, pod crash loops, and mesh/tunnel issues where applicable. (pending)
- Ensure the rollout plan in `distributed-k3s` only reports, guards, and coordinates with K3s rather than bypassing the K3s scheduler. (pending)
- Recheck the full `distributed-k3s` boundary so no command accidentally turns the agent into a second hidden control plane. (pending)

## Day 30 - Hardening and Acceptance Gate

Goal: close the agent track with acceptance coverage, docs, and a clear gap summary.

- Run the acceptance matrix for enrollment, standalone rollout, mesh routing, sidecar precedence, telemetry, and scale-to-zero. (pending)
- Run rollback tests, mesh failure tests, wake timeout tests, topology/reporting tests, and security review for redaction and logging. (pending)
- Complete docs, runbooks, local dev guide, mock contract guide, and the backend/CLI/frontend handoff checklist. (pending)
- Summarize remaining gaps, technical debt, and v2 items that are outside this 30-day baseline. (pending)

## Mandatory Test Plan

- Bootstrap: reject `expired token`, reject `reused token`, bind enrollment to the correct target, and mark the instance online after enrollment. (pending)
- Standalone: `push` creates a deployment, health failure blocks promotion, and rollback returns traffic to the previous stable revision. (pending)
- Mesh: peer join/leave is clean, a service on node A can reach a dependency on node B through the private path, and mesh failure marks topology as degraded. (pending)
- Sidecar: `env injection`, `managed credential injection`, and `localhost rescue` all respect precedence for local and cross-node routing, and no secret is logged. (pending)
- K3s: `node_agent` reports logs and node metrics, K3s environment detection works correctly, and LazyOps does not `docker run` workloads in `distributed-k3s`. (pending)
- Observability: every public request gets `X-Correlation-ID`, trace summary shows the correct service path, and metric rollup correctly computes `p95`, `max`, `min`, `avg`, and `count`. (pending)
- Scale-to-zero: only opt-in services may sleep, the first request after sleep wakes successfully, and gateway hold timeout is enforced. (pending)

## Assumptions

- The 30 days are understood as 30 consecutive working days for the agent track.
- All tasks in the file start in `(pending)` status; only the task currently being worked on should be moved to `(doing)`.
- The `node_agent` scope in these 30 days is telemetry, topology, logs, metrics, and cluster incident reporting for `distributed-k3s`; it does not expand into a scheduler replacing K3s.
- If backend contracts are not ready yet, the agent still uses mocks that match the locked schemas and must not invent fields, command names, or payload shapes that conflict with the master plan.
- If the source guides change later, this tracker must be updated to reflect the new guides before related tasks continue being marked `(done)`.
