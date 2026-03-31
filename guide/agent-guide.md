# Agent Guide

## 1. Role

LazyOps Agent is the runtime bridge between the central control plane and the user's infrastructure. It must support multiple runtime modes without changing the user experience.

The product uses two main agent forms:

- `instance_agent` for `standalone` and `distributed-mesh`
- `node_agent` for `distributed-k3s`

Both agents connect outbound only. No persistent inbound control port is required.

## 2. Agent Modes

### 2.1 Instance Agent

Used when:

- one server maps to one deployment target
- or multiple servers are connected by mesh VPN without Kubernetes

Responsibilities:

- runtime execution through a local runtime driver
- Caddy gateway management
- compatibility sidecar lifecycle
- mesh VPN peer management
- rollout health checks
- rollback handling
- scale-to-zero sleep and wake-up
- edge log filtering and metrics rollup

### 2.2 Node Agent

Used when:

- the user chooses `distributed-k3s`
- K3s is responsible for workload scheduling

Responsibilities:

- run as `DaemonSet`
- collect container logs from node paths
- collect node-level metrics
- emit topology and incident data
- support tunnel relay and trace signals

Non-responsibilities:

- no direct app workload placement
- no direct `docker run` for user app workloads
- no competing scheduler behavior

## 3. Shared Modules

- `connection manager`
- `agent auth`
- `bootstrap token exchange`
- `heartbeat and capability reporter`
- `incident reporter`
- `edge metrics rollup`
- `trace summary reporter`
- `tunnel relay`

## 4. Instance Agent Modules

- `runtime driver`
- `gateway manager`
- `sidecar manager`
- `mesh manager`
- `deployment manager`
- `health gate`
- `rollback handler`
- `autosleep manager`
- `log collector`
- `service metadata cache`

## 5. Node Agent Modules

- `daemonset bootstrap`
- `k3s environment detector`
- `container log tailer`
- `node metrics collector`
- `pod topology reporter`
- `cluster incident reporter`

## 6. Bootstrap Without Stored SSH

### 6.1 Preferred install flow

1. User creates an instance or node record.
2. Backend issues a short-lived bootstrap token.
3. User runs a one-time install command on the target machine.
4. Agent enrolls and exchanges bootstrap token for a long-lived agent token.
5. Bootstrap token is invalidated.

### 6.2 Why this matters

This model means:

- no persistent SSH storage in LazyOps
- no repeated SSH deploy channel
- deploys happen over the agent control session instead

## 7. Mesh VPN

### 7.1 Providers

- default: `WireGuard`
- optional: `Tailscale`

### 7.2 Required behavior

- create encrypted overlay connectivity between participating servers
- assign internal service-reachable addresses
- keep peer cleanup deterministic
- never log plaintext mesh secrets

### 7.3 Mode boundaries

- `standalone`: mesh manager is disabled
- `distributed-mesh`: mesh manager is active and routes service-to-service calls
- `distributed-k3s`: mesh is still useful for node or cluster private connectivity, but workload orchestration belongs to K3s

## 8. Compatibility Sidecar and Smart Proxy

The sidecar is a core component, not an optional helper.

It must support:

- environment injection when the app respects env-based config
- managed credential injection for internal dependencies
- `localhost` rescue when the app hard-codes local endpoints
- forwarding to local targets or remote mesh targets
- correlation propagation
- latency measurement per hop

Precedence:

1. env injection
2. managed credentials
3. localhost rescue

## 9. Localhost Illusion

The product goal is that a lazy application can still connect correctly.

Examples:

- app calls `localhost:5432` for Postgres
- app calls `localhost:5672` for RabbitMQ
- app calls `localhost:8080` for another internal service

Expected result:

- sidecar intercepts the call inside the same network namespace
- resolves the dependency binding
- forwards locally or through mesh VPN
- keeps the user away from manual port or credential mapping

## 10. Runtime and Rollout

### 10.1 Standalone and distributed-mesh

Deploy flow:

1. receive desired revision
2. pull image
3. prepare sidecars and gateway config
4. start candidate workload
5. wait for health checks
6. shift traffic
7. stop or drain previous revision

### 10.2 Distributed-k3s

Deploy flow:

1. receive desired revision intent
2. report node capability and local health
3. K3s applies workload changes
4. node agent reports rollout and failure signals

### 10.3 Global rollback

If a promoted revision becomes unstable:

1. emit incident
2. mark revision unhealthy
3. backend reverts desired state
4. traffic returns to previous stable revision

## 11. Scale-to-Zero

### 11.1 Behavior

- service is stopped after configured idle window
- metadata remains available for wake-up
- gateway holds the first new request briefly
- agent wakes the service and resumes the request

### 11.2 Support order

- first-class in `standalone`
- first-class in `distributed-mesh`
- staged in `distributed-k3s` after baseline rollout and rollback support

## 12. Observability

### 12.1 Logs

- hot path should prefer byte matching over heavy regex
- cooldown windows must prevent alert storms
- only relevant excerpts should be sent to the control plane when possible

### 12.2 Metrics

Agent computes edge windows such as:

- `p95`
- `max`
- `min`
- `avg`
- `count`

### 12.3 Topology and tracing

Agent reports:

- service placement
- node health
- mesh link health
- gateway health
- trace hop summary
- local latency signals

## 13. Performance Budget

- idle RAM target under 20 MB where practical
- idle CPU target under 2 percent
- sidecar hot path must avoid unnecessary allocations
- tunnel relay should use buffer pooling

## 14. Parallel Work Plan

### 14.1 Agent team tracks

- bootstrap and token exchange
- connection manager and heartbeat
- runtime driver for standalone and mesh
- mesh manager
- sidecar and gateway lifecycle
- node agent for K3s
- logs, metrics, traces, and incidents
- scale-to-zero wake-up logic

### 14.2 Contracts needed early

- agent handshake schema
- desired revision schema
- deployment binding payload
- command envelope
- trace summary payload
- incident payload
- scale-to-zero policy contract
