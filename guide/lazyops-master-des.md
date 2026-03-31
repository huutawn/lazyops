# LazyOps Master Design

## 1. Product Promise

LazyOps is a self-hosted deployment platform focused on lazy operations:

- the user registers servers once
- the user links a GitHub repository once
- the user runs `lazyops init` once to generate a deploy contract
- after that, `git push` or `pull_request` becomes the main deployment trigger

The product promise is:

- code push should be enough to trigger build and rollout
- users do not need to hand-write infrastructure YAML
- users do not need to remember IPs, ports, or server-specific credentials
- services should still work even when the app hard-codes `localhost`
- the same product must work for one server and for distributed multi-server systems

## 2. Three Operating Modes

### 2.1 Standalone

- one server
- one registered instance
- no Kubernetes requirement
- instance agent manages runtime, gateway, sidecar, logs, metrics, and wake/sleep

### 2.2 Distributed Mesh

- multiple servers
- services may be placed on different instances
- agents establish encrypted private connectivity with `WireGuard` or `Tailscale`
- compatibility sidecar and mesh proxy route service-to-service traffic across servers

### 2.3 Distributed K3s

- only used when the user wants a real distributed scheduler
- K3s handles scheduling, restart, node failover, and scaling
- LazyOps still owns developer experience, bindings, sidecar policy, tracing, topology, and FinOps

## 3. Core Building Blocks

### 3.1 Central Server

The control plane owns:

- authentication and identity
- GitHub App integration and webhook handling
- project, service, blueprint, and deployment state
- desired state revision management
- build orchestration with Nixpacks
- topology graph, traces, incidents, and FinOps aggregates

### 3.2 Build Plane

Builds happen in LazyOps-managed infrastructure:

- clone by short-lived GitHub App installation token
- inspect `lazyops.yaml`
- build with Nixpacks
- push artifact metadata back to control plane

The builder never needs SSH access to the user's servers.

### 3.3 Agent Plane

Agent responsibilities depend on runtime mode:

- `instance_agent` for `standalone` and `distributed-mesh`
- `node_agent` for `distributed-k3s`

Shared responsibilities:

- outbound control connection
- heartbeat and capability report
- incident reporting
- edge metrics rollup
- tunnel relay
- trace and topology signals

### 3.4 Sidecar and Mesh Proxy

LazyOps sidecar is a first-class runtime component.

It must handle:

- environment injection for well-behaved apps
- managed credentials for internal dependencies
- `localhost` interception for hard-coded apps
- cross-server forwarding across the mesh
- request timing and correlation propagation

## 4. How Deploy Works

### 4.1 Login and Repo Access

- user identity can come from email/password, Google OAuth2, or GitHub OAuth2
- repo access should come from a GitHub App, not from stored personal tokens

### 4.2 Instance Registration Without Stored SSH

LazyOps should prefer:

- one-time bootstrap commands
- short-lived bootstrap tokens
- copy-paste install scripts
- optional one-time SSH bootstrap that discards credentials immediately after install

Long-term deploys happen over agent sessions, not over SSH.

### 4.3 `lazyops init`

`init` is the CLI onboarding step that:

- scans the local repository
- detects candidate services
- fetches available `instances`, `mesh networks`, and `clusters`
- lets the user choose the deployment target mode
- creates a logical `lazyops.yaml`

`init` does not embed secrets. It writes references such as:

- project slug
- runtime mode
- service list
- binding target reference
- dependency bindings
- health checks
- magic domain policy
- scale-to-zero policy

### 4.4 Git Push to Deploy

After onboarding:

1. GitHub emits `push` or `pull_request`.
2. LazyOps receives the event through the GitHub App webhook.
3. Builder clones the relevant commit and reads `lazyops.yaml`.
4. Backend resolves the logical binding to actual instance, mesh, or cluster targets.
5. Backend creates a desired state revision.
6. Agents or K3s converge runtime state to match that revision.

## 5. URL and Networking Model

- public URLs default to `https://service-name.<public-ip>.sslip.io`
- fallback is `https://service-name.<public-ip>.nip.io`
- public ingress is handled by `Caddy Gateway`
- internal service-to-service traffic goes through local sidecars and, when needed, through mesh VPN links

## 6. Reliability Model

- zero-downtime rollout is the default
- traffic shifts only after the new revision passes health checks
- rollback can be local or global depending on target mode
- `scale-to-zero` is opt-in and should keep request-hold behavior in gateway before wake-up

## 7. Observability Model

LazyOps must make distributed failures understandable:

- `X-Correlation-ID` is injected at the gateway
- sidecars and agents emit hop summaries
- latency hotspots are aggregated by node and by service edge
- topology is shown in React Flow
- logs open directly from the topology or deployment views

## 8. Cost Model

FinOps is built into the platform:

- edge downsampling for CPU and RAM
- storage-friendly hourly summaries using `p95`, `max`, `min`, `avg`, `count`
- runtime cost suggestions from actual utilization
- idle service sleep policies where safe

## 9. Product Boundaries

LazyOps should not:

- require users to learn Kubernetes first
- require users to manage SSH after onboarding
- require raw infrastructure YAML as the main UX
- expose internal databases directly to the internet
- depend on manual per-repo CI setup in the user's repository

## 10. Parallel Delivery Principle

The product must be buildable by multiple teams in parallel:

- backend publishes contracts early
- agent implements runtime adapters and telemetry
- CLI implements onboarding and local operator flows
- frontend builds screens from mock contracts and progressively swaps to real APIs
