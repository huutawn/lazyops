# Agent Day 1 Baseline Audit

## Summary

Day 1 confirms that the current agent codebase is still at a very early baseline. The `agent/` directory currently contains only an empty `go.mod` file and a `main.go` stub with `package main`. There is no runtime implementation, no enrollment flow, no outbound control session, no shared contracts, no sidecar logic, no mesh manager, no telemetry pipeline, and no `node_agent` mode yet.

Day 1 conclusions:

- The agent track should be treated as a near-greenfield implementation inside the current repo.
- Day 1 deliverables are documentation and architecture locks, not runtime code yet.
- The next execution order remains correct: lock contracts first, then build shared runtime foundations, then `instance_agent`, then `distributed-mesh`, then sidecar and telemetry depth, and finally `node_agent` telemetry for `distributed-k3s`.
- The agent must stay aligned to the product rules from the start so later implementation does not drift into `K3s-first`, SSH-based deployment, or ad-hoc runtime behavior.

## Source Documents

- `guide/agent-guide.md`
- `guide/lazyops-implementation-master-plan.md`
- `guide/lazyops-master-des.md`
- `guide/project-rules.md`
- `agent-task.md`

## Current Agent Inventory

### Files present today

- `agent/go.mod`
- `agent/cmd/server/main.go`

### Current file reality

- `agent/go.mod`: empty file, `0` bytes.
- `agent/cmd/server/main.go`: contains only `package main`.

### What currently exists

- A placeholder directory structure for the future agent binary.
- No usable Go module definition.
- No internal packages.
- No config loading.
- No bootstrap flow.
- No local state store.
- No enrollment logic.
- No control-plane transport.
- No heartbeat or capability reporting.
- No command dispatcher.
- No runtime driver abstraction.
- No gateway, sidecar, mesh, rollout, rollback, autosleep, logging, metrics, tracing, topology, incident, or tunnel implementation.
- No tests.

### Gap audit against `agent-guide.md`

| Guide area | Required capability | Current state | Gap summary |
| --- | --- | --- | --- |
| Agent role | runtime bridge across all runtime modes | missing | agent does not exist as a working runtime bridge yet |
| Agent forms | `instance_agent` and `node_agent` | missing | no mode split or mode detector |
| Shared modules | connection, auth, bootstrap, heartbeat, incidents, metrics, traces, tunnel | missing | none of the shared modules exist |
| Instance modules | runtime, gateway, sidecar, mesh, deployment, health, rollback, autosleep, logs, metadata cache | missing | all instance-agent modules are unimplemented |
| Node modules | daemonset bootstrap, K3s detector, container logs, node metrics, topology, incidents | missing | all node-agent modules are unimplemented |
| Bootstrap flow | short-lived bootstrap token exchange | missing | no enrollment protocol or token persistence |
| Outbound-only control | no inbound persistent control port | missing | no control session exists yet |
| Mesh VPN | `WireGuard` baseline, optional `Tailscale`, deterministic cleanup | missing | no mesh provider or route sync exists |
| Sidecar | env injection, managed credentials, localhost rescue | missing | no sidecar lifecycle or compatibility layer exists |
| Rollout | candidate start, health gate, promote, rollback | missing | no runtime deploy flow exists |
| Scale-to-zero | sleep, wake, gateway hold/resume | missing | no autosleep or wake path exists |
| Observability | log filtering, metric rollup, topology, trace summary | missing | no telemetry pipeline exists |
| Performance budget | low idle RAM/CPU, low-allocation hot path | missing | no code exists yet to enforce or measure budget |
| Parallel delivery | early contracts for backend/CLI/frontend | missing | no agent-side contract package or schema exists |

## Day 1 Compliance Checklist

This checklist locks the rules and implementation boundaries that Day 2+ work must follow.

| Area | Locked rule or boundary | Day 1 status | Day 1 decision |
| --- | --- | --- | --- |
| Runtime modes | support `standalone`, `distributed-mesh`, `distributed-k3s` | locked | all later agent work must keep the 3-mode product model |
| Agent split | `instance_agent` only for `standalone` and `distributed-mesh` | locked | instance runtime code belongs only to these two modes |
| Agent split | `node_agent` only for `distributed-k3s` | locked | node mode is telemetry/protection only |
| K3s boundary | K3s is installed only when user chooses `distributed-k3s` | locked | no K3s assumptions in baseline agent runtime |
| K3s boundary | LazyOps must not bypass K3s for user workload placement | locked | `node_agent` must never directly schedule app workloads |
| Public contract | user workloads remain modeled as `service`, not `container` | locked | runtime abstractions must expose service-level interfaces |
| Control-plane posture | zero-inbound, outbound-only agent connection | locked | primary transport is outbound control session only |
| SSH posture | no long-lived SSH for normal operations | locked | enrollment uses bootstrap token exchange, not stored SSH |
| Enrollment security | prefer short-lived bootstrap token or one-time install flow | locked | bootstrap exchange is mandatory in early contracts |
| Secret handling | never log plaintext PAT, agent token, GitHub token, mesh key, or managed secret | locked | redaction and secure storage are required from Day 3 onward |
| Internal exposure | internal databases, queues, and private services stay off public ports by default | locked | gateway planning must preserve private defaults |
| Source of truth | `lazyops.yaml` stores logical intent only | locked | agent must never rely on repo-stored infrastructure secrets |
| Target authority | backend resolves target through `DeploymentBinding` | locked | agent consumes binding payload; it does not invent target authority |
| Sidecar role | compatibility sidecar is core, not optional | locked | sidecar manager is a required runtime subsystem |
| Sidecar precedence | `env injection -> managed credential injection -> localhost rescue` | locked | all compatibility handling must preserve this order |
| Private networking | cross-node private traffic must use `WireGuard` or `Tailscale` | locked | no public-route fallback for internal service traffic |
| Public ingress | default gateway is `Caddy`; magic domains prefer `sslip.io`, fallback `nip.io` | locked | gateway module must preserve this domain and ingress model |
| Reliability | zero-downtime rollout is default | locked | candidate/promote/drain flow is mandatory |
| Reliability | global rollback must exist | locked | runtime must preserve stable fallback revision state |
| Scale-to-zero | opt-in only | locked | autosleep must always check policy guardrails |
| Correlation | every public request must receive `X-Correlation-ID` | locked | agent/gateway/sidecar tracing must propagate this header |
| Metrics | metrics must be downsampled at the edge | locked | agent sends rollups, not raw long-term samples |
| Logs | hot-path filtering should prefer byte matching over heavy regex | locked | log collector design must bias to low-cost matching |
| Tracing | summarized tracing is preferred over raw full-fidelity-only capture | locked | trace reporter must send summarized hop data |
| Parallel delivery | contracts must be published early so backend, CLI, and frontend can move in parallel | locked | Day 2 contract work is a hard dependency, not optional cleanup |
| Kubernetes control-plane boundary | agent must never become a second hidden control plane fighting Kubernetes | locked | `node_agent` remains reporting/protection oriented |

## Locked Agent Module Map

Day 1 locks the high-level module map below so the rest of the roadmap can use one stable structure.

### Shared modules

| Module | Suggested package area | Responsibility |
| --- | --- | --- |
| connection manager | `internal/control` | outbound websocket control session, reconnect, keepalive, disconnect handling |
| agent auth | `internal/auth` | session auth, token load/store, auth headers, control-session auth helpers |
| bootstrap token exchange | `internal/enroll` | enrollment request/response, bootstrap exchange, bootstrap retry and invalidation handling |
| heartbeat and capability reporter | `internal/heartbeat` | periodic liveness, capability snapshot, online/degraded/busy state emission |
| incident reporter | `internal/incidents` | runtime incident capture and transport to backend |
| edge metrics rollup | `internal/metrics` | edge aggregation, metric windows, rollup emission |
| trace summary reporter | `internal/tracing` | correlation-aware hop summary generation and upload |
| tunnel relay | `internal/tunnel` | debug tunnel relay and signal relay support |
| shared contracts | `internal/contracts` | command envelope, payload schemas, status enums, ack/error envelopes |
| local state store | `internal/state` | agent metadata, enrollment state, revision cache, telemetry buffers |
| process bootstrap | `internal/app` and `internal/config` | config loading, lifecycle boot, signal handling, graceful shutdown |

### Instance agent modules

| Module | Suggested package area | Responsibility |
| --- | --- | --- |
| runtime driver | `internal/instance/runtime` | local runtime abstraction for service lifecycle and revision actions |
| gateway manager | `internal/instance/gateway` | render/apply/rollback Caddy config, domain and TLS wiring |
| sidecar manager | `internal/instance/sidecar` | sidecar lifecycle, dependency proxy config, compatibility policy |
| mesh manager | `internal/instance/mesh` | peer state, routes, provider integration, mesh health |
| deployment manager | `internal/instance/deploy` | reconcile revision flow, candidate state, promote/drain orchestration |
| health gate | `internal/instance/health` | rollout health checks and health thresholds |
| rollback handler | `internal/instance/rollback` | stable revision restore and rollback triggers |
| autosleep manager | `internal/instance/autosleep` | idle detection, sleep/wake orchestration, gateway hold/resume coordination |
| log collector | `internal/instance/logs` | runtime/gateway/sidecar log filtering and excerpt forwarding |
| service metadata cache | `internal/instance/servicecache` | service placement, alias resolution, dependency target cache |

### Node agent modules

| Module | Suggested package area | Responsibility |
| --- | --- | --- |
| daemonset bootstrap | `internal/node/bootstrap` | assets and runtime assumptions for DaemonSet mode |
| k3s environment detector | `internal/node/k3sdetect` | mode detection and K3s environment validation |
| container log tailer | `internal/node/logtail` | collect logs from node/container paths |
| node metrics collector | `internal/node/metrics` | collect node-level metrics and emit rollups |
| pod topology reporter | `internal/node/topology` | pod/node topology snapshots and node health reporting |
| cluster incident reporter | `internal/node/incidents` | cluster-side incident emission without bypassing K3s scheduling |

### Structural decisions

- Shared contracts and shared state must be implemented before mode-specific runtime logic.
- `instance_agent` and `node_agent` should share transport, auth, state, metrics, tracing, and tunnel layers where practical.
- Mode-specific code must stay under separate package areas so `distributed-k3s` boundaries remain obvious in code review.
- No module should assume that the backend is already complete; mocks that match locked contracts are part of the intended design.

## Locked Agent State Machine

Day 1 locks the agent state machine below for the rest of the roadmap.

### States

| State | Meaning | Entry conditions | Exit conditions |
| --- | --- | --- | --- |
| `bootstrap` | process startup and local initialization | process start, restart, or re-enrollment requirement | move to `connected`, `degraded`, or `disconnected` |
| `connected` | steady-state online control session | control channel authenticated and heartbeat active | move to `reconciling`, `reporting`, `sleeping`, `degraded`, or `disconnected` |
| `reconciling` | desired-state convergence or command execution | revision reconcile, mesh sync, gateway/sidecar apply, or wake/sleep action | move to `connected`, `degraded`, or `disconnected` |
| `degraded` | agent is alive but one or more subsystems are impaired | local subsystem failure, health/reporting failure, wake failure, mesh impairment | move to `connected`, `reporting`, or `disconnected` |
| `sleeping` | service workload is sleeping but the agent remains active | autosleep policy has put a service into sleep state | move to `connected`, `degraded`, or `reporting` |
| `reporting` | transient state for flushing telemetry or incidents | heartbeat, topology, incident, trace, or metric flush is actively happening | move to `connected`, `sleeping`, or `degraded` |
| `disconnected` | no active control session with backend | network loss, auth failure, backend unavailable, or explicit disconnect | move to `connected` or `bootstrap` |

### Required transitions

- `bootstrap -> connected`: local state is valid, enrollment is complete, and the outbound control session is authenticated.
- `bootstrap -> degraded`: local startup is partially successful but one or more required subsystems are impaired and recoverable.
- `bootstrap -> disconnected`: the agent cannot establish or restore a control session after bootstrap attempts.
- `connected -> reconciling`: a control command arrives or local desired-state drift must be reconciled.
- `connected -> reporting`: scheduled or event-driven telemetry must be flushed immediately.
- `connected -> sleeping`: autosleep policy has suspended the service workload while keeping the agent online.
- `connected -> degraded`: runtime, gateway, sidecar, mesh, or telemetry subsystem becomes impaired.
- `connected -> disconnected`: control session is lost or auth becomes invalid.
- `reconciling -> connected`: reconcile or command flow finishes successfully.
- `reconciling -> degraded`: reconcile fails in a recoverable way and requires incident/reporting or local retry.
- `reconciling -> disconnected`: control session is lost during reconcile.
- `degraded -> reporting`: an incident, trace, metric, or topology report must be emitted while degraded.
- `degraded -> connected`: failed subsystem recovers and steady-state operation is restored.
- `degraded -> disconnected`: control session is lost while already impaired.
- `sleeping -> connected`: a wake request or held gateway request resumes the service successfully.
- `sleeping -> degraded`: wake-up fails or required metadata for wake-up is inconsistent.
- `sleeping -> reporting`: sleep/wake telemetry or incident data must be emitted.
- `reporting -> connected`: telemetry flush succeeds and steady-state connection continues.
- `reporting -> sleeping`: telemetry flush completes while the service remains in sleep mode.
- `reporting -> degraded`: repeated reporting failures cause local impairment.
- `disconnected -> connected`: re-authentication and reconnect succeed.
- `disconnected -> bootstrap`: local auth state is invalid or missing and re-enrollment is required.

### State machine notes

- `sleeping` applies to the managed service workload, not to the agent process itself.
- `reporting` is intentionally transient and must not block reconnect, wake-up, or rollback behavior.
- `degraded` is not a crash state; the agent should continue preserving local state, incidents, and buffered telemetry where possible.
- `disconnected` must preserve token secrecy, local state integrity, and eventual reconnect behavior.

## Day 1 Exit Decisions

- Day 1 is complete when the tracker reflects that audit, compliance checklist, module map, and state machine work are all locked.
- No Day 2+ work should rename modules, invent new runtime states, or weaken the locked boundaries without first updating this document and the source tracker.
- The next concrete implementation work should begin with the Day 2 contract package and control-session schema locking.
