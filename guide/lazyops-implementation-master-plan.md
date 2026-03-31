# LazyOps Master Implementation Spec

## 0. Document Purpose

This file is the main execution spec for LazyOps.

It is the source of truth for:

- backend
- agent
- CLI
- frontend
- build pipeline
- auth and repo integration

It is not a README. It is a delivery document for teams that need to build in parallel.

---

## 1. System Goals

### 1.1 Product promise

LazyOps exists to make self-hosted deployment feel lazy:

- users register servers once
- users connect GitHub once
- users initialize the repo once
- after that, `git push` and `pull_request` drive build and rollout

The user should not need to:

- manage long-lived SSH access from LazyOps
- hand-write infrastructure YAML
- memorize server IPs, ports, or internal credentials
- classify services as frontend or backend just to deploy

### 1.2 Runtime modes

LazyOps must support three operating modes.

| Mode | Meaning | Scheduler | Agent behavior | Typical user |
| --- | --- | --- | --- | --- |
| `standalone` | One server, one deployment target | none | `instance_agent` manages local runtime | solo app, SME, MVP |
| `distributed-mesh` | Many servers connected by private mesh, no Kubernetes requirement | none | many `instance_agent` nodes plus mesh routing | split workloads across VPSs |
| `distributed-k3s` | Many servers plus K3s | K3s | `node_agent` reports and protects, K3s schedules workloads | larger team, higher resilience need |

### 1.3 Service model

Every user workload is modeled as a `service`.

Rules:

- LazyOps does not require a frontend/backend distinction
- multiple services may run on one instance
- one service may depend on many other services
- service-to-service communication is solved by env injection, compatibility sidecar, and mesh routing

### 1.4 What `init` is for

`lazyops init` is the onboarding command that turns a repository into a stable deploy contract.

It must:

- scan the repo
- detect services
- fetch available targets from backend
- let the user choose runtime mode and target
- create or attach a logical `DeploymentBinding`
- write `lazyops.yaml`

It must not:

- store SSH credentials
- write raw infrastructure secrets
- depend on the builder knowing how to SSH into servers

### 1.5 Definition of done

LazyOps v1 is successful when:

1. A user can log in with email/password, Google OAuth2, or GitHub OAuth2.
2. A user can register a server without LazyOps storing long-lived SSH access.
3. A user can connect a GitHub repo through GitHub App.
4. `lazyops init` can generate a valid `lazyops.yaml`.
5. `git push` triggers Nixpacks build and deploy to the user's target.
6. `pull_request` can create preview environments.
7. `localhost` and bad internal config can still be rescued by sidecar or env injection.
8. Distributed mode can connect services across servers over mesh VPN.
9. Public URLs work out of the box with `sslip.io` or `nip.io` and HTTPS via Caddy.
10. Logs, traces, topology, incidents, and metrics are visible in UI and CLI.

---

## 2. Non-Negotiable Rules

### 2.1 Security and ownership

- `RULE 01`: Zero-inbound is the default posture. Agents connect outbound to LazyOps.
- `RULE 02`: LazyOps must not store long-lived SSH private keys for normal operations.
- `RULE 03`: PAT, agent tokens, GitHub tokens, mesh keys, and managed secrets must never be logged in plaintext.
- `RULE 04`: Internal databases, queues, and private services must not expose public ports by default.

### 2.2 Runtime boundaries

- `RULE 05`: K3s is installed only when the user chooses `distributed-k3s`.
- `RULE 06`: In `distributed-k3s`, Kubernetes is the workload orchestrator. LazyOps must not bypass it to manage user workloads directly.
- `RULE 07`: In `standalone` and `distributed-mesh`, LazyOps may use a local runtime driver behind the scenes, but the public contract remains `service`.

### 2.3 Source of truth

- `RULE 08`: `lazyops.yaml` is the project deploy contract.
- `RULE 09`: `lazyops.yaml` stores logical intent only. It must not store SSH, raw kubeconfig, server passwords, or private keys.
- `RULE 10`: Mapping from repo to actual deployment target must be resolved by backend through `DeploymentBinding`.

### 2.4 GitHub integration

- `RULE 11`: User identity may come from Google OAuth2 or GitHub OAuth2, but repo access must prefer GitHub App.
- `RULE 12`: Build and webhook ownership must use short-lived GitHub App installation tokens whenever possible.
- `RULE 13`: LazyOps should not require writing CI files into the user's repository by default.

### 2.5 Networking and compatibility

- `RULE 14`: Compatibility sidecar is a core runtime feature.
- `RULE 15`: Internal connectivity precedence is `env injection`, then `managed credential injection`, then `localhost rescue`.
- `RULE 16`: In multi-server deployments, private service traffic must go through `WireGuard` or `Tailscale`.
- `RULE 17`: Public ingress must support magic domains, preferring `sslip.io` and falling back to `nip.io`.

### 2.6 Reliability and observability

- `RULE 18`: Zero-downtime rollout is the default deployment policy.
- `RULE 19`: Global rollback must exist for unhealthy promoted revisions.
- `RULE 20`: `scale-to-zero` is opt-in, not globally forced.
- `RULE 21`: Every inbound public request must receive an `X-Correlation-ID`.
- `RULE 22`: Metrics must be downsampled at the edge before long-term storage.

### 2.7 Delivery model

- `RULE 23`: Backend, agent, CLI, and frontend must be buildable in parallel from published contracts.
- `RULE 24`: Frontend must be able to use mocks until runtime implementation is ready.
- `RULE 25`: CLI must be able to onboard targets without waiting for frontend completion.

---

## 3. Current State vs Target State

The project does not fully satisfy the desired product criteria yet. The gaps are the reason for this spec revision.

| Area | Current direction problem | Target direction |
| --- | --- | --- |
| Runtime mode | previous docs leaned too much toward K3s-first | hybrid `standalone`, `distributed-mesh`, `distributed-k3s` |
| Init purpose | previously treated mostly as config generator | scan repo, fetch targets, create binding, write deploy contract |
| Deploy trigger | earlier docs still mixed CLI-centric deploy assumptions | `git push` and `pull_request` are the main triggers |
| Repo access | earlier drafts were ambiguous about OAuth vs repo permissions | OAuth for identity, GitHub App for repo and webhook access |
| Server targeting | repo could still be misunderstood as containing target knowledge | repo stores logical `target_ref`; backend resolves real targets |
| SSH dependency | not clearly removed everywhere | bootstrap may use one-time enrollment, no long-lived SSH storage |
| K3s boundary | old sections could imply Kubernetes everywhere | K3s only in `distributed-k3s` |
| Sidecar role | sidecar existed but was not consistently central | sidecar is core for env injection, localhost rescue, and mesh forwarding |
| Frontend scope | no dedicated guide existed | dedicated `frontend-guide.md` and parallel track |
| Domain model | missing or inconsistent models for bindings and installations | `Instance`, `MeshNetwork`, `DeploymentBinding`, `GitHubInstallation` are first-class |

---

## 4. Target Architecture

### 4.1 Architectural layers

The system has five layers:

1. `Control Plane`
2. `Build Plane`
3. `Runtime Plane`
4. `Data Plane`
5. `Experience Plane`

### 4.2 Control Plane

Owned by backend.

Responsibilities:

- auth and identity
- project, service, target, and binding state
- blueprint compilation
- desired state revision management
- deployment orchestration
- topology and trace aggregation
- incident and metrics ingestion
- GitHub App and webhook integration

### 4.3 Build Plane

Owned by LazyOps-controlled build workers.

Responsibilities:

- clone source by GitHub App installation token
- inspect `lazyops.yaml`
- build with Nixpacks
- push artifact metadata back to backend

Non-responsibilities:

- no SSH into user infrastructure
- no long-lived repo secrets in worker

### 4.4 Runtime Plane

Depends on runtime mode.

`standalone`:

- one `instance_agent`
- local runtime driver
- local Caddy gateway
- local sidecar injection

`distributed-mesh`:

- many `instance_agent` nodes
- private overlay networking
- cross-node service routing
- optional topology-driven placement

`distributed-k3s`:

- K3s schedules workloads
- `node_agent` runs as `DaemonSet`
- LazyOps focuses on policy, reporting, sidecar config, and desired revision generation

### 4.5 Data Plane

Carries:

- HTTP and WebSocket traffic
- sidecar-proxied service traffic
- mesh VPN traffic
- log, metrics, incident, and trace telemetry

### 4.6 Experience Plane

User-facing surfaces:

- web app
- CLI
- GitHub integration

These must tell the same story:

- register target
- connect repo
- initialize deploy contract
- push code
- observe runtime

### 4.7 Responsibility matrix

| Concern | Standalone | Distributed-mesh | Distributed-k3s |
| --- | --- | --- | --- |
| App workload placement | instance agent | instance agents + placement rules | K3s |
| Mesh private connectivity | no | yes | optional or cluster-native |
| Public gateway | local Caddy | one or more Caddy gateways | Caddy or cluster ingress strategy |
| Sidecar compatibility | yes | yes | yes |
| Deployment orchestration | backend + instance agent | backend + instance agents | backend + K3s API + node agents |
| Auto-healing | limited by runtime driver | limited by runtime driver and policy | K3s native |
| Scale-to-zero | yes | yes | staged after baseline support |

---

## 5. Identity, Auth, and Repo Permissions

### 5.1 Auth methods

Supported:

- email/password
- Google OAuth2
- GitHub OAuth2
- PAT for CLI
- agent token for agents

### 5.2 OAuth2 purpose

Google OAuth2 and GitHub OAuth2 exist for user identity and smoother onboarding.

They are not the primary long-lived mechanism for:

- repo clone
- webhook management
- CI/CD job authorization

### 5.3 GitHub App purpose

GitHub App is the default integration for:

- repository discovery
- installation-scoped access
- webhook ownership
- short-lived installation tokens
- build status and checks

### 5.4 Required GitHub App permissions

Minimum:

- `Metadata: Read`
- `Contents: Read`
- `Pull requests: Read`
- `Commit statuses: Read/Write`
- `Checks: Read/Write` if richer UI is desired

### 5.5 Auth flow plan

#### Web login

1. User selects email/password, Google, or GitHub login.
2. Backend authenticates.
3. Backend issues web session JWT or secure cookie.

#### CLI login

1. User runs `lazyops login`.
2. CLI opens browser or accepts credentials.
3. Backend returns PAT through `POST /api/v1/auth/cli-login`.
4. CLI stores PAT in keychain or protected local file.

#### GitHub App link

1. User installs LazyOps GitHub App into repo or org.
2. Backend syncs installations.
3. User links repository to project.
4. Backend records repository ownership and installation mapping.

### 5.6 Why repo clone does not need SSH

Repo clone happens with a short-lived GitHub App installation token.

Deploy targeting happens with:

- `ProjectRepoLink`
- `DeploymentBinding`
- online agent sessions or cluster driver

This means:

- the builder can clone code
- the builder still cannot deploy arbitrarily
- only backend can resolve actual target and initiate rollout

---

## 6. Server and Target Onboarding Without Long-Lived SSH

### 6.1 Preferred target enrollment

Every target should be enrolled using a short-lived bootstrap token.

Flow:

1. User creates an `Instance` or `Cluster` record.
2. Backend issues a bootstrap token with short TTL.
3. User runs a copy-paste install command on the server.
4. Agent exchanges bootstrap token for agent token.
5. Backend invalidates bootstrap token.

### 6.2 Optional one-time SSH bootstrap

If a later product flow uses SSH for convenience:

- SSH credentials must be short-lived
- they must be discarded immediately after agent install
- normal deployments must never depend on stored SSH

### 6.3 Why this satisfies the product promise

After agent enrollment:

- deploys happen over agent control sessions
- builders never need SSH
- repo never contains server secrets
- users still get lazy deploy UX

---

## 7. `lazyops.yaml` Deploy Contract

### 7.1 Design goals

The file must be:

- human-reviewable
- stable enough to live in the repo
- safe to clone in builder
- rich enough for backend to compile desired state

### 7.2 File purpose

`lazyops.yaml` tells LazyOps:

- what services exist
- what runtime mode the project expects
- what logical binding to use
- what compatibility policies are enabled
- what health checks and domains should exist

It does not tell LazyOps:

- SSH secrets
- raw kubeconfig
- exact deployable private credentials

### 7.3 Minimum schema

```yaml
project_slug: acme-shop
runtime_mode: distributed-mesh

deployment_binding:
  target_ref: prod-ap

services:
  - name: web
    path: apps/web
    start_hint: npm run start
    public: true
    healthcheck:
      path: /health
      port: 3000
  - name: api
    path: apps/api
    public: false
    healthcheck:
      path: /healthz
      port: 8080

dependency_bindings:
  - service: api
    alias: postgres
    target_service: app-db
    protocol: tcp
    local_endpoint: localhost:5432
  - service: web
    alias: api
    target_service: api
    protocol: http
    local_endpoint: localhost:8080

compatibility_policy:
  env_injection: true
  managed_credentials: true
  localhost_rescue: true

magic_domain_policy:
  enabled: true
  provider: sslip.io

preview_policy:
  enabled: true

scale_to_zero_policy:
  enabled: false
```

### 7.4 How builder uses the file

Builder reads:

- repo layout
- service list
- runtime mode
- target reference

Builder does not resolve:

- which instance is active
- which agents are online
- where secrets live

That resolution stays in backend.

### 7.5 Why `target_ref` works

`target_ref` is a logical key that points to a backend-managed `DeploymentBinding`.

Example:

- repo says `target_ref: prod-ap`
- backend maps `prod-ap` to mesh network `mesh-prod-ap` and instances `[sg-1, hn-1]`
- rollout goes to currently valid targets only

If the same repo is cloned outside LazyOps, it still cannot deploy because actual deployment authority lives in backend state and agent sessions.

---

## 8. Domain Model

### 8.1 Identity and access

#### `User`

- `id`
- `email`
- `password_hash`
- `display_name`
- `status`
- `created_at`
- `updated_at`

#### `OAuthIdentity`

- `id`
- `user_id`
- `provider` (`google`, `github`)
- `provider_subject`
- `email`
- `avatar_url`
- `created_at`
- `updated_at`

#### `PersonalAccessToken`

- `id`
- `user_id`
- `name`
- `token_hash`
- `last_used_at`
- `expires_at`
- `revoked_at`
- `created_at`

#### `GitHubInstallation`

- `id`
- `user_id`
- `github_installation_id`
- `account_login`
- `account_type`
- `scope_json`
- `installed_at`
- `revoked_at`

### 8.2 Targets

#### `Instance`

- `id`
- `user_id`
- `name`
- `public_ip`
- `private_ip`
- `agent_id`
- `status`
- `labels_json`
- `runtime_capabilities_json`
- `created_at`

#### `MeshNetwork`

- `id`
- `user_id`
- `name`
- `provider` (`wireguard`, `tailscale`)
- `cidr`
- `status`
- `created_at`

#### `Cluster`

- `id`
- `user_id`
- `name`
- `provider` (`k3s`)
- `kubeconfig_secret_ref`
- `status`
- `created_at`

#### `DeploymentBinding`

- `id`
- `project_id`
- `name`
- `target_ref`
- `runtime_mode`
- `target_kind` (`instance`, `mesh`, `cluster`)
- `target_id`
- `placement_policy_json`
- `domain_policy_json`
- `compatibility_policy_json`
- `scale_to_zero_policy_json`
- `created_at`

### 8.3 Projects and revisions

#### `Project`

- `id`
- `user_id`
- `name`
- `slug`
- `default_branch`
- `created_at`

#### `ProjectRepoLink`

- `id`
- `project_id`
- `github_installation_id`
- `github_repo_id`
- `repo_owner`
- `repo_name`
- `tracked_branch`
- `preview_enabled`
- `created_at`

#### `Service`

- `id`
- `project_id`
- `name`
- `path`
- `public`
- `runtime_profile`
- `healthcheck_json`
- `created_at`

#### `Blueprint`

- `id`
- `project_id`
- `source_kind` (`lazyops_yaml`, `ui_plan`, `api_update`)
- `source_ref`
- `compiled_json`
- `created_at`

#### `DesiredStateRevision`

- `id`
- `project_id`
- `blueprint_id`
- `deployment_binding_id`
- `commit_sha`
- `trigger_kind` (`push`, `pull_request`, `manual`)
- `status`
- `compiled_revision_json`
- `created_at`

#### `Deployment`

- `id`
- `project_id`
- `revision_id`
- `status`
- `started_at`
- `completed_at`

#### `PreviewEnvironment`

- `id`
- `project_id`
- `revision_id`
- `pr_number`
- `status`
- `domain_json`
- `destroyed_at`

### 8.4 Runtime visibility

#### `TraceSummary`

- `id`
- `project_id`
- `correlation_id`
- `trace_json`
- `started_at`
- `ended_at`

#### `NodeLatencyReport`

- `id`
- `project_id`
- `source_node_id`
- `target_node_id`
- `service_edge`
- `p95_ms`
- `max_ms`
- `window_started_at`
- `window_ended_at`

#### `MetricRollup`

- `id`
- `project_id`
- `target_kind`
- `target_id`
- `service_name`
- `window`
- `cpu_p95`
- `cpu_max`
- `ram_p95`
- `ram_max`
- `sample_count`

#### `RuntimeIncident`

- `id`
- `project_id`
- `revision_id`
- `severity`
- `kind`
- `summary`
- `details_json`
- `created_at`

#### `TopologyNode`

- `id`
- `project_id`
- `node_type`
- `node_ref`
- `status`
- `metadata_json`

#### `TopologyEdge`

- `id`
- `project_id`
- `source_node_ref`
- `target_node_ref`
- `edge_type`
- `status`
- `latency_summary_json`

---

## 9. Interface Contracts

### 9.1 Public HTTP APIs

#### Auth and identity

- `POST /api/v1/auth/register`
- `POST /api/v1/auth/login`
- `POST /api/v1/auth/cli-login`
- `POST /api/v1/auth/pat/revoke`
- `GET /api/v1/auth/oauth/google/start`
- `GET /api/v1/auth/oauth/google/callback`
- `GET /api/v1/auth/oauth/github/start`
- `GET /api/v1/auth/oauth/github/callback`

#### GitHub and repos

- `POST /api/v1/github/app/installations/sync`
- `GET /api/v1/github/repos`
- `POST /api/v1/projects/:id/repo-link`
- `POST /api/v1/integrations/github/webhook`

#### Targets

- `POST /api/v1/instances`
- `GET /api/v1/instances`
- `POST /api/v1/mesh-networks`
- `GET /api/v1/mesh-networks`
- `POST /api/v1/clusters`
- `GET /api/v1/clusters`
- `POST /api/v1/projects/:id/deployment-bindings`

#### Project and runtime

- `POST /api/v1/projects`
- `GET /api/v1/projects`
- `PUT /api/v1/projects/:id/blueprint`
- `POST /api/v1/projects/:id/deployments`
- `GET /api/v1/projects/:id/topology`
- `GET /api/v1/traces/:correlation_id`
- `POST /api/v1/tunnels/db/sessions`

### 9.2 WebSocket and streaming endpoints

- `GET /ws/operators/stream`
- `GET /ws/agents/control`
- `GET /ws/logs/stream`

### 9.3 Build callback contract

```json
{
  "build_job_id": "bld_123",
  "project_id": "prj_123",
  "commit_sha": "abc123",
  "status": "succeeded",
  "image_ref": "registry.example.com/acme/web:abc123",
  "image_digest": "sha256:deadbeef",
  "metadata": {
    "detected_services": ["web", "api"]
  }
}
```

### 9.4 Command envelope

All internal commands and events should use a shared envelope.

```json
{
  "type": "deploy.start_candidate",
  "request_id": "req_123",
  "correlation_id": "corr_123",
  "agent_id": "agt_123",
  "project_id": "prj_123",
  "source": "backend",
  "occurred_at": "2026-03-31T12:00:00Z",
  "payload": {}
}
```

### 9.5 Agent command set

Minimum commands:

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

### 9.6 Operator event set

- `deployment.started`
- `deployment.build_failed`
- `deployment.candidate_ready`
- `deployment.promoted`
- `deployment.rolled_back`
- `incident.created`
- `trace.recorded`
- `topology.updated`
- `metric.rollup_ingested`

---

## 10. Function Catalog

Each function below defines purpose, input, output, validation, side effects, errors, and permission.

### 10.1 Auth

#### Function: `Web Login`

- Purpose: authenticate user for web UI.
- Input: email/password or OAuth callback data.
- Output: session JWT or secure session cookie.
- Validation: valid credentials, valid OAuth state and callback.
- Side effects: create or update session record, update last login.
- Error cases: bad password, revoked account, invalid OAuth state.
- Permission: public.

#### Function: `CLI Login`

- Purpose: issue PAT for CLI use.
- Input: authenticated user context.
- Output: PAT token and metadata.
- Validation: user must already authenticate successfully.
- Side effects: create PAT record.
- Error cases: rate limit, disabled account.
- Permission: public after successful auth flow.

#### Function: `PAT Revoke`

- Purpose: revoke a CLI token.
- Input: token id or current token.
- Output: revoke confirmation.
- Validation: token belongs to current user.
- Side effects: token marked revoked.
- Error cases: token not found, token belongs to another user.
- Permission: authenticated user.

### 10.2 GitHub and repo link

#### Function: `Sync GitHub Installations`

- Purpose: fetch GitHub App installations available to current user.
- Input: user session plus installation sync request.
- Output: installation list.
- Validation: GitHub OAuth identity or GitHub App callback context is valid.
- Side effects: update local installation records.
- Error cases: missing GitHub identity, revoked installation, API error.
- Permission: authenticated user.

#### Function: `Link Repository`

- Purpose: bind a repo to a project.
- Input: project id, installation id, repo id, tracked branch.
- Output: `ProjectRepoLink`.
- Validation: user owns project, installation grants repo access.
- Side effects: persist repo link, enable webhook routing.
- Error cases: repo not accessible, branch invalid, ownership mismatch.
- Permission: project owner or admin.

### 10.3 Target enrollment

#### Function: `Create Instance`

- Purpose: register a future deployment target.
- Input: instance name, public IP, labels.
- Output: instance record and bootstrap token.
- Validation: user owns tenant, input is well formed.
- Side effects: create instance record, create bootstrap token.
- Error cases: duplicate name, invalid IP.
- Permission: authenticated user.

#### Function: `Enroll Agent`

- Purpose: exchange bootstrap token for agent token.
- Input: bootstrap token, machine info, capabilities.
- Output: agent token, agent id.
- Validation: bootstrap token exists, is unexpired, and belongs to correct target.
- Side effects: bind agent to target, invalidate bootstrap token.
- Error cases: expired token, already used token, ownership mismatch.
- Permission: bootstrap-token scoped.

### 10.4 Deploy contract and binding

#### Function: `Generate lazyops.yaml`

- Purpose: create deploy contract from repo scan and user target choice.
- Input: local repository scan result, runtime mode, target selection, dependency declarations.
- Output: `lazyops.yaml`.
- Validation: service names unique, target exists, runtime mode valid.
- Side effects: optional `DeploymentBinding` creation in backend.
- Error cases: unknown target, invalid dependency mapping.
- Permission: authenticated CLI or web user.

#### Function: `Create DeploymentBinding`

- Purpose: create logical mapping from project to actual target.
- Input: project id, target kind, target id, runtime mode, binding name.
- Output: `DeploymentBinding`.
- Validation: user owns both project and target, mode is compatible with target.
- Side effects: persist binding, make it selectable by CLI and UI.
- Error cases: incompatible mode, duplicate target ref, target offline if strict validation enabled.
- Permission: project owner or admin.

### 10.5 Build and rollout

#### Function: `Start Build Job`

- Purpose: start a Nixpacks build when webhook arrives.
- Input: repo link, event payload, commit SHA.
- Output: build job record.
- Validation: webhook signature valid, repo is linked, branch policy allows trigger.
- Side effects: enqueue build worker task.
- Error cases: invalid signature, repo not linked, unsupported event.
- Permission: GitHub App webhook.

#### Function: `Compile Blueprint`

- Purpose: combine `lazyops.yaml`, project state, and target state into desired runtime document.
- Input: project id, build artifact, repo contract, binding.
- Output: `Blueprint` and `DesiredStateRevision`.
- Validation: contract valid, binding exists, required services resolvable.
- Side effects: create revision records.
- Error cases: invalid contract, missing target, missing artifact.
- Permission: backend internal.

#### Function: `Apply Revision`

- Purpose: start candidate rollout to selected runtime driver.
- Input: desired state revision id.
- Output: deployment record and initial rollout state.
- Validation: target reachable, artifacts available, health policies valid.
- Side effects: agent commands or K3s API calls, gateway config update plan.
- Error cases: no online agents, cluster unreachable, artifact missing.
- Permission: backend internal or authorized manual deploy.

#### Function: `Rollback Revision`

- Purpose: return traffic and desired state to last known stable revision.
- Input: project id, revision id, rollback reason.
- Output: rollback record.
- Validation: previous stable revision exists.
- Side effects: update desired state, shift traffic, emit incident.
- Error cases: no stable fallback, gateway unavailable.
- Permission: backend internal or project operator.

### 10.6 Sidecar and networking

#### Function: `Resolve DependencyBinding`

- Purpose: determine how one service reaches another dependency.
- Input: service name, alias, target service, runtime mode, placement.
- Output: env injection values or proxy route.
- Validation: dependency target exists, protocol supported.
- Side effects: config generation for sidecar and gateway.
- Error cases: unsupported protocol, missing target.
- Permission: backend internal.

#### Function: `Open Debug Tunnel`

- Purpose: create secure local operator tunnel to internal service.
- Input: project id, service target, local port.
- Output: tunnel session record.
- Validation: caller owns project and has access to target.
- Side effects: relay setup through agent and backend.
- Error cases: target offline, port unavailable, permission denied.
- Permission: project operator.

### 10.7 Observability and FinOps

#### Function: `Record Trace Summary`

- Purpose: store a summarized distributed trace.
- Input: correlation id, hop data, latency data.
- Output: trace summary record.
- Validation: payload shape valid, project ownership valid.
- Side effects: topology and incident correlation.
- Error cases: malformed payload, missing project mapping.
- Permission: agent or gateway internal.

#### Function: `Ingest Metric Rollup`

- Purpose: store edge-downsampled metrics.
- Input: windowed aggregates.
- Output: metric rollup record.
- Validation: aggregate schema valid.
- Side effects: FinOps dashboards and alerts become updatable.
- Error cases: malformed aggregate, target not found.
- Permission: agent internal.

---

## 11. Use Case Catalog

Each group contains at least two happy paths and one failure path.

### 11.1 Auth

#### Use case: Web login with Google

- Actor: user.
- Precondition: Google OAuth configured.
- Happy path:
  1. user clicks Google login
  2. backend redirects to Google
  3. callback returns successfully
  4. session is created
- Failure path: OAuth state mismatch or revoked Google account.
- Done condition: user reaches dashboard authenticated.

#### Use case: CLI login once

- Actor: developer.
- Precondition: account exists.
- Happy path:
  1. developer runs `lazyops login`
  2. backend authenticates
  3. PAT is issued
  4. CLI stores PAT
- Failure path: PAT creation blocked by disabled account or bad credentials.
- Done condition: later CLI commands run without re-entering password.

#### Use case: Revoke PAT

- Actor: user.
- Precondition: PAT exists.
- Happy path:
  1. user revokes token
  2. backend marks token revoked
  3. CLI must re-login next time
- Failure path: token belongs to different account.
- Done condition: revoked token no longer authenticates.

### 11.2 Target onboarding

#### Use case: Register standalone instance

- Actor: operator.
- Precondition: user is logged in.
- Happy path:
  1. user creates instance in UI
  2. backend issues bootstrap token
  3. user runs install command on server
  4. agent enrolls
- Failure path: bootstrap token expires before install.
- Done condition: instance appears online.

#### Use case: Register distributed mesh nodes

- Actor: operator.
- Precondition: at least two servers exist.
- Happy path:
  1. user enrolls multiple instances
  2. backend groups them into mesh network
  3. agents establish private connectivity
- Failure path: one node cannot join mesh due to connectivity issue.
- Done condition: mesh health is visible and usable.

#### Use case: Attach cluster for distributed-k3s

- Actor: operator.
- Precondition: K3s cluster exists.
- Happy path:
  1. cluster record is created
  2. credentials are validated
  3. node agents appear
- Failure path: kube API unreachable or invalid credentials.
- Done condition: cluster is available as deployment target.

### 11.3 Init and binding

#### Use case: Generate deploy contract for standalone

- Actor: developer.
- Precondition: repo exists and CLI is logged in.
- Happy path:
  1. CLI scans repo
  2. CLI fetches available instances
  3. user selects standalone target
  4. CLI writes `lazyops.yaml`
- Failure path: no valid target exists.
- Done condition: repo contains valid deploy contract.

#### Use case: Generate deploy contract for distributed mesh

- Actor: developer.
- Precondition: mesh network exists.
- Happy path:
  1. CLI scans repo
  2. CLI detects multiple services
  3. user selects distributed mesh
  4. binding is created
  5. `lazyops.yaml` is written
- Failure path: selected mesh is not owned by the project user.
- Done condition: repo points to logical target ref only.

#### Use case: Invalid dependency binding

- Actor: developer.
- Precondition: service graph contains typo or unsupported protocol.
- Happy path: validation catches issue before write or before deploy.
- Failure path: invalid dependency target reaches backend and is rejected.
- Done condition: no broken desired revision is compiled.

### 11.4 GitHub and build

#### Use case: Repo link and push deploy

- Actor: operator and GitHub App.
- Precondition: project exists and GitHub App installed.
- Happy path:
  1. user links repo
  2. user pushes to tracked branch
  3. webhook arrives
  4. build worker clones commit
  5. Nixpacks builds artifact
  6. backend compiles revision
  7. rollout starts
- Failure path: webhook signature invalid.
- Done condition: deployment record exists for pushed commit.

#### Use case: PR preview environment

- Actor: GitHub App.
- Precondition: preview policy enabled.
- Happy path:
  1. pull request opens
  2. webhook arrives
  3. build runs
  4. preview revision is created
  5. preview URL is published
- Failure path: preview target capacity unavailable.
- Done condition: PR has a reachable preview environment.

#### Use case: PR close teardown

- Actor: GitHub App.
- Precondition: preview environment exists.
- Happy path:
  1. `pull_request.closed` arrives
  2. backend resolves preview environment
  3. runtime destroys preview resources
- Failure path: target already offline during teardown.
- Done condition: preview environment is removed cleanly.

### 11.5 Sidecar and networking

#### Use case: Standard env injection

- Actor: runtime.
- Precondition: app respects env-based dependency configuration.
- Happy path:
  1. backend resolves dependency binding
  2. runtime injects correct connection values
  3. app connects directly
- Failure path: env contract missing required key.
- Done condition: app reaches dependency without manual user wiring.

#### Use case: Localhost rescue across servers

- Actor: sidecar.
- Precondition: app hard-codes `localhost`.
- Happy path:
  1. sidecar intercepts local call
  2. sidecar resolves target service
  3. call is forwarded through mesh VPN
- Failure path: mesh route unhealthy.
- Done condition: app still reaches remote dependency.

#### Use case: Magic domain URL provisioning

- Actor: gateway.
- Precondition: target has valid public IP.
- Happy path:
  1. backend allocates public route
  2. Caddy config is updated
  3. HTTPS cert is obtained
- Failure path: public IP invalid for magic domain provider.
- Done condition: service has public HTTPS URL.

### 11.6 Rollout and resilience

#### Use case: Zero-downtime rollout

- Actor: backend, gateway, agent.
- Precondition: previous stable revision exists.
- Happy path:
  1. candidate starts
  2. health checks pass
  3. traffic shifts to candidate
  4. previous revision drains
- Failure path: candidate never becomes healthy.
- Done condition: no visible downtime during promotion.

#### Use case: Global rollback

- Actor: backend and runtime.
- Precondition: a stable previous revision exists.
- Happy path:
  1. promoted revision becomes unhealthy
  2. incident fires
  3. desired state reverts to previous revision
  4. traffic returns to stable revision
- Failure path: previous stable revision unavailable.
- Done condition: uptime is restored quickly.

#### Use case: Scale-to-zero wake-up

- Actor: gateway and agent.
- Precondition: service opted into scale-to-zero and is sleeping.
- Happy path:
  1. request arrives
  2. gateway holds request briefly
  3. agent wakes service
  4. request resumes
- Failure path: service fails to wake within timeout.
- Done condition: request reaches service after cold start.

### 11.7 Observability and FinOps

#### Use case: Trace a failed user request

- Actor: operator.
- Precondition: request went through gateway and sidecars.
- Happy path:
  1. operator opens trace by correlation id
  2. UI shows service hops and latency
  3. failing hop is visible
- Failure path: some agents fail to report trace summaries.
- Done condition: operator understands where the request broke.

#### Use case: Detect latency bottleneck

- Actor: backend analytics.
- Precondition: node and edge latency reports exist.
- Happy path:
  1. reports arrive
  2. backend computes hot edges
  3. UI marks slow edge or node
- Failure path: sparse data causes low-confidence result.
- Done condition: likely bottleneck is visible.

#### Use case: Store long-term cheap metrics

- Actor: agent and backend.
- Precondition: metrics rollup pipeline is active.
- Happy path:
  1. agent computes edge aggregates
  2. backend stores only aggregates
  3. UI renders long-window charts cheaply
- Failure path: aggregate payload malformed.
- Done condition: long-term metrics stay lightweight.

---

## 12. Detailed Roadmap

This roadmap is designed so teams can move in parallel.

### Milestone 0 - Contract Reset

Goal:

- align every guide and spec around hybrid runtime and logical deploy binding

Deliverables:

- rewritten master spec
- aligned rules
- backend, agent, CLI, and frontend guides

Exit:

- no guide still implies K3s for all deployments
- no guide still implies repo-stored deploy secrets

### Milestone 1 - Identity and GitHub Foundation

Goal:

- establish all auth and repo ownership paths

Deliverables:

- email/password login
- Google OAuth2 login
- GitHub OAuth2 login
- PAT for CLI
- GitHub App install sync
- repo link model
- webhook verification

Dependencies:

- DB tables for `OAuthIdentity`, `PAT`, `GitHubInstallation`, `ProjectRepoLink`

Exit:

- user can log in
- user can install GitHub App
- user can link repo to project

### Milestone 2 - Target Enrollment and Binding

Goal:

- make server and target registration work without stored SSH

Deliverables:

- `Instance`, `MeshNetwork`, `Cluster` CRUD
- bootstrap token issue and exchange
- agent enrollment
- `DeploymentBinding` CRUD
- target list APIs for CLI and frontend

Exit:

- a target can be enrolled and selected by `init`

### Milestone 3 - CLI Init and Contract Generation

Goal:

- make `lazyops init` produce a real deploy contract

Deliverables:

- repo scanner
- service detector
- target chooser
- binding creation flow
- `lazyops.yaml` generator
- `lazyops doctor`

Exit:

- repo can be initialized without writing secrets

### Milestone 4 - Build Plane and Nixpacks

Goal:

- build from webhook-triggered commits

Deliverables:

- build job queue
- installation-token-based clone
- Nixpacks builder
- build callback endpoint
- artifact metadata storage

Exit:

- `push` can produce a build artifact

### Milestone 5 - Standalone Runtime Baseline

Goal:

- make one-server deployments fully functional first

Deliverables:

- `instance_agent`
- local runtime driver
- Caddy gateway management
- sidecar injection
- health gate
- zero-downtime rollout
- rollback

Exit:

- one server can receive `git push` driven deployments

### Milestone 6 - Distributed Mesh Runtime

Goal:

- add multi-server routing without forcing Kubernetes

Deliverables:

- mesh network management
- `WireGuard` provider baseline
- optional `Tailscale` adapter slot
- cross-node dependency binding resolution
- topology reporting
- mesh-aware rollout planner

Exit:

- a service on server A can reach a dependency on server B privately

### Milestone 7 - Compatibility Sidecar and Localhost Rescue

Goal:

- make bad internal config survivable

Deliverables:

- env injection
- managed credential injection
- localhost rescue
- protocol routing for `http` and `tcp`
- latency measurement in sidecar

Exit:

- app can hard-code `localhost` and still function through sidecar

### Milestone 8 - Public URLs, Preview, and Rollout UX

Goal:

- make product feel complete for real teams

Deliverables:

- magic domain URL provisioning
- HTTPS via Caddy
- PR preview environments
- preview cleanup on close
- deployment history and release detail APIs

Exit:

- push and PR are fully visible and usable

### Milestone 9 - Distributed K3s Mode

Goal:

- support Kubernetes only when user actually wants it

Deliverables:

- cluster registration
- K3s runtime driver
- `node_agent` `DaemonSet`
- cluster-side telemetry
- K3s-backed rollout plan

Exit:

- user can choose K3s as an advanced distributed mode

### Milestone 10 - Observability and Topology

Goal:

- make failures debuggable across services and nodes

Deliverables:

- `X-Correlation-ID` propagation
- trace summary storage
- node latency report ingestion
- topology graph API
- React Flow topology UI
- log stream and drilldown

Exit:

- operator can trace and visualize request flow

### Milestone 11 - FinOps and Scale-to-Zero

Goal:

- cut cost and make metrics affordable

Deliverables:

- edge downsampling
- long-window metric storage
- FinOps views
- scale-to-zero for standalone
- scale-to-zero for distributed-mesh

Exit:

- idle workloads can sleep where policy allows
- metrics stay storage-efficient

### Milestone 12 - Hardening and Acceptance Gate

Goal:

- ensure the whole system is production-credible

Deliverables:

- security test matrix
- webhook trust validation
- target ownership validation
- rollback tests
- mesh failure tests
- preview cleanup tests
- docs and runbooks

Exit:

- product passes acceptance matrix for v1

---

## 13. Parallel Execution Plan by Team

### 13.1 Backend team can start immediately on

- auth and OAuth
- GitHub App integration
- target and binding schema
- build queue and callback
- blueprint compiler
- topology and trace APIs

### 13.2 Agent team can start immediately on

- bootstrap token exchange
- heartbeat and capability report
- instance agent runtime abstraction
- mesh manager abstraction
- sidecar runtime
- node agent telemetry mode

### 13.3 CLI team can start immediately on

- login and PAT storage
- repo scan
- service detection
- target selection
- `lazyops.yaml` generation
- logs and trace commands

### 13.4 Frontend team can start immediately on

- auth screens
- onboarding flow
- targets list
- repo integration UI
- deployment history
- React Flow topology canvas
- metrics and trace views

### 13.5 Shared contracts that must be locked early

- auth response models
- target summary models
- deployment binding schema
- `lazyops.yaml` schema
- desired revision schema
- topology node and edge schema
- trace summary schema
- metric rollup schema

---

## 14. Test Plan

### 14.1 Auth and identity

- wrong password rejected
- Google OAuth2 state mismatch rejected
- GitHub OAuth2 callback ownership validated
- revoked PAT no longer works
- user A cannot access user B project or targets

### 14.2 GitHub App and webhook

- invalid webhook signature rejected
- unlinked repo ignored
- installation token clone succeeds
- PR open creates preview flow
- PR close destroys preview flow

### 14.3 Target enrollment

- expired bootstrap token rejected
- reused bootstrap token rejected
- agent enrollment binds to correct target only
- instance appears online after enrollment

### 14.4 Deploy contract and binding

- invalid `lazyops.yaml` rejected
- unknown `target_ref` rejected
- mismatched runtime mode and target kind rejected
- builder can read contract without gaining deploy rights

### 14.5 Standalone rollout

- push to tracked branch creates deployment
- health failure blocks promotion
- rollback returns traffic to last stable revision
- magic domain URL resolves after successful rollout

### 14.6 Distributed mesh

- mesh join and leave work cleanly
- service on node A reaches service on node B through private path
- localhost rescue still works across nodes
- mesh failure marks topology degraded

### 14.7 Distributed-k3s

- K3s target validation works
- node agent reports logs and node metrics
- LazyOps does not bypass K3s for app scheduling

### 14.8 Observability

- every public request receives `X-Correlation-ID`
- trace summary shows actual service path
- bottleneck report marks slow node or edge
- topology graph matches runtime state

### 14.9 FinOps

- agent sends only rolled-up metrics
- DB does not store raw high-volume samples for long-term history
- `p95`, `max`, `min`, `avg`, `count` are computed correctly

### 14.10 Scale-to-zero

- opted-in service sleeps after idle threshold
- first request after sleep wakes service
- gateway hold timeout is enforced
- critical services remain excluded when policy disabled

---

## 15. Assumptions and Non-Goals

### 15.1 Assumptions

- runtime modes are `standalone`, `distributed-mesh`, `distributed-k3s`
- K3s is optional and appears only in distributed advanced mode
- `sslip.io` is preferred and `nip.io` is fallback
- GitHub App is the default repo integration model
- build uses Nixpacks in LazyOps-controlled infrastructure
- `scale-to-zero` is first implemented in standalone and distributed-mesh

### 15.2 Non-goals for initial v1

- full multi-cloud autoscheduler
- provider-specific managed database provisioning
- requiring raw Kubernetes YAML authoring by users
- long-lived SSH-managed deployment model

---

## 16. Immediate Build Order

The recommended execution order for the codebase is:

1. auth + OAuth + PAT
2. GitHub App install sync + webhook verification
3. `Instance`, `MeshNetwork`, `Cluster`, `DeploymentBinding` schema
4. agent bootstrap token flow
5. CLI `init` + `lazyops.yaml`
6. Nixpacks build worker
7. standalone runtime deployment
8. distributed mesh routing
9. sidecar compatibility
10. preview environments
11. topology, traces, metrics, FinOps
12. distributed-k3s mode

This order keeps the product usable early while still leaving room for the distributed advanced mode later in the roadmap.
