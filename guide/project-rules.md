# LazyOps Project Rules

This file defines the non-negotiable rules for the whole product. Every guide and every implementation plan must follow these rules.

## 1. Runtime Modes

- `RULE 01`: LazyOps must support three product modes: `standalone`, `distributed-mesh`, and `distributed-k3s`.
- `RULE 02`: `K3s` is installed only when the user explicitly chooses `distributed-k3s`.
- `RULE 03`: `standalone` means one server, one instance target, no Kubernetes requirement.
- `RULE 04`: `distributed-mesh` means multiple servers connected by mesh VPN, without forcing Kubernetes.
- `RULE 05`: `distributed-k3s` means multiple servers plus K3s for scheduling, auto-healing, and scaling.

## 2. Responsibility Split

- `RULE 06`: In `distributed-k3s`, Kubernetes is the workload orchestrator. LazyOps must not bypass K3s to `docker run`, `docker stop`, or directly schedule user workloads.
- `RULE 07`: In `standalone` and `distributed-mesh`, LazyOps may use a local runtime driver behind the scenes, but the public product contract is always `service`, not `container`.
- `RULE 08`: LazyOps owns developer experience, deployment contracts, sidecar compatibility, mesh routing, observability, and FinOps.
- `RULE 09`: The agent never becomes a second hidden control plane that fights Kubernetes.

## 3. Security

- `RULE 10`: Zero-inbound remains the default security posture. Agents connect outbound to the control plane.
- `RULE 11`: LazyOps must not persist SSH private keys for long-term operations.
- `RULE 12`: Instance registration should prefer bootstrap tokens, one-time install commands, or short-lived enrollment secrets over stored SSH access.
- `RULE 13`: PAT, agent tokens, GitHub tokens, mesh keys, and managed secrets must be stored hashed or encrypted at rest and never logged in plaintext.
- `RULE 14`: Databases, queues, and internal services must not be exposed on public ports by default.

## 4. Source of Truth

- `RULE 15`: `lazyops.yaml` is the project deploy contract. It contains logical deployment intent, not raw infrastructure secrets.
- `RULE 16`: `lazyops.yaml` must never store SSH credentials, server passwords, private keys, raw kubeconfig, or long-lived GitHub credentials.
- `RULE 17`: Repo-to-target resolution must happen through backend-managed `DeploymentBinding`, not by hard-coded IPs in the repository.
- `RULE 18`: All user code units are modeled as `services`. The product must not force a frontend/backend distinction.

## 5. GitHub and Identity

- `RULE 19`: User identity may come from email/password, Google OAuth2, or GitHub OAuth2.
- `RULE 20`: GitHub OAuth2 is for identity and optional repo discovery only; the default repo access and webhook model must use a GitHub App.
- `RULE 21`: Build and deploy flow must not require writing CI files into the user's repository by default.
- `RULE 22`: Build jobs must clone with a short-lived GitHub App installation token and build with Nixpacks in LazyOps-controlled infrastructure.

## 6. Networking and Compatibility

- `RULE 23`: The compatibility sidecar is a core product feature, not a debug-only helper.
- `RULE 24`: The sidecar must support three precedence levels: `env injection`, `managed credential injection`, and `localhost rescue`.
- `RULE 25`: When services on different servers communicate, the product must establish a private overlay path through `WireGuard` or `Tailscale`.
- `RULE 26`: Magic domains must be supported out of the box, preferring `sslip.io` and falling back to `nip.io`.
- `RULE 27`: Public HTTPS termination should be handled by `Caddy Gateway` by default.

## 7. Reliability

- `RULE 28`: Zero-downtime rollout is the default deployment policy unless the service explicitly opts out.
- `RULE 29`: Global rollback must be supported when a new revision becomes unhealthy after promotion.
- `RULE 30`: `scale-to-zero` is supported, but only as an opt-in policy per service or per environment.

## 8. Observability and FinOps

- `RULE 31`: Every request entering the public gateway must receive an `X-Correlation-ID`.
- `RULE 32`: Distributed tracing must be summarized from gateway, sidecar, and agent events rather than from raw full-fidelity capture only.
- `RULE 33`: Metrics must be downsampled at the edge before being sent to the central server for long-term storage.
- `RULE 34`: Hot-path log filtering must avoid expensive regex when cheaper byte matching is sufficient.

## 9. Parallel Delivery

- `RULE 35`: Backend, agent, CLI, and frontend must be able to move in parallel from published contracts and mock data.
- `RULE 36`: Frontend must not be blocked on real runtime implementation to build auth, topology, deployment history, traces, and FinOps views.
- `RULE 37`: CLI `init` must work against published APIs for `instances`, `mesh-networks`, `clusters`, `projects`, and `deployment-bindings`.
- `RULE 38`: The master implementation spec is the execution source of truth; the other guides derive from it.
