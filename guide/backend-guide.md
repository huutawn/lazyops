# Backend Guide

## 1. Role

Backend is the central control plane. It owns product state, auth, repository integration, desired state compilation, deployment orchestration, observability aggregation, and FinOps rollups.

Backend does not build inside the user's repository and does not rely on stored SSH access for normal deployments.

## 2. Main Responsibilities

- identity and auth
- PAT lifecycle for CLI
- agent and node registration
- GitHub App integration and webhook normalization
- build request orchestration through Nixpacks build workers
- project, service, and deployment binding management
- blueprint and desired state revision management
- runtime dispatch to `standalone`, `distributed-mesh`, or `distributed-k3s`
- topology graph, trace summaries, incident stream, and metrics ingest

## 3. Module Layout

### 3.1 Core modules

- `internal/auth`
- `internal/oauth`
- `internal/pat`
- `internal/github`
- `internal/projects`
- `internal/services`
- `internal/instances`
- `internal/mesh`
- `internal/clusters`
- `internal/bindings`
- `internal/blueprint`
- `internal/deployments`
- `internal/build`
- `internal/runtime`
- `internal/topology`
- `internal/tracing`
- `internal/metrics`
- `internal/incidents`
- `internal/tunnels`

### 3.2 Runtime driver interface

The backend must expose a common interface so the rest of the system does not care whether the target is a single server, a mesh, or K3s.

```go
type RuntimeDriver interface {
    ValidateTarget(ctx context.Context, target DeploymentTarget) error
    Plan(ctx context.Context, req DeployRequest) (DeployPlan, error)
    Apply(ctx context.Context, plan DeployPlan) (ApplyResult, error)
    Watch(ctx context.Context, plan DeployPlan) (RolloutResult, error)
    Rollback(ctx context.Context, req RollbackRequest) error
}
```

Drivers:

- `standalone`: talks to one `instance_agent`
- `distributed-mesh`: talks to multiple `instance_agent` nodes plus mesh coordination
- `distributed-k3s`: talks to K3s API and cluster resources; it must not bypass Kubernetes for workload placement

## 4. Auth and Identity

### 4.1 Supported auth methods

- email and password
- Google OAuth2
- GitHub OAuth2
- PAT for CLI
- agent token for agents

### 4.2 Responsibility split

- Google OAuth2 and GitHub OAuth2 are for user identity
- PAT is for CLI session continuity
- agent token is for runtime enrollment and control channel auth
- GitHub App installation token is for repo clone, webhook ownership, and build operations

### 4.3 OAuth flow plan

Google OAuth2:

1. Frontend hits `GET /api/v1/auth/oauth/google/start`.
2. Backend redirects to Google.
3. Google calls back to `GET /api/v1/auth/oauth/google/callback`.
4. Backend finds or creates the user and issues web session JWT.

GitHub OAuth2:

1. Frontend hits `GET /api/v1/auth/oauth/github/start`.
2. Backend redirects to GitHub OAuth consent.
3. GitHub calls back to `GET /api/v1/auth/oauth/github/callback`.
4. Backend finds or creates the user and issues web session JWT.

CLI login:

1. User runs `lazyops login`.
2. CLI opens browser for OAuth or submits email/password.
3. Backend returns PAT through `POST /api/v1/auth/cli-login`.
4. CLI stores PAT in OS keychain if available, else a local protected file.

## 5. GitHub Integration

### 5.1 Why GitHub App is the default

GitHub App is the preferred model because it gives:

- short-lived installation tokens
- repository-scoped access
- first-class webhook ownership
- cleaner revocation than user PATs
- support for org-level install and repo selection

### 5.2 Recommended permissions

- `Metadata: Read`
- `Contents: Read`
- `Pull requests: Read`
- `Commit statuses: Read and Write`
- `Checks: Read and Write` if richer build UI is desired
- `Webhooks`: installation-level delivery

### 5.3 Repo link flow

1. User logs in to LazyOps.
2. User installs LazyOps GitHub App on selected repo or organization.
3. Backend syncs installations via `POST /api/v1/github/app/installations/sync`.
4. Frontend or CLI shows installable repositories.
5. User links one repo to one LazyOps project.
6. Backend stores `ProjectRepoLink` and allowed branches.

### 5.4 Webhook flow

1. GitHub App emits `push`, `pull_request`, and `pull_request.closed`.
2. Backend verifies signature and normalizes payload.
3. Backend resolves `project_id`, `repo_id`, `branch`, and `commit_sha`.
4. Backend starts a build job.
5. Build worker clones using an installation token and builds with Nixpacks.
6. Build worker calls backend callback with artifact metadata.
7. Backend compiles a blueprint revision and dispatches rollout.

## 6. No-SSH Deployment Resolution

### 6.1 The problem

Builder must know where to deploy, but the repository must not contain SSH credentials or server secrets.

### 6.2 The answer

Use a logical binding model:

- `lazyops.yaml` stores `project_slug`, `runtime_mode`, and `deployment_binding.target_ref`
- backend stores `DeploymentBinding`
- `DeploymentBinding` resolves `target_ref` to actual target objects

This means:

- cloned code can describe intent
- only backend can resolve real deploy targets
- deploy authorization depends on backend state plus online agent sessions

### 6.3 Example

`lazyops.yaml` may contain:

```yaml
project_slug: acme-shop
runtime_mode: distributed-mesh
deployment_binding:
  target_ref: prod-mesh-ap
services:
  - name: web
  - name: api
```

Backend resolves `prod-mesh-ap` to:

- mesh network `mesh-prod-ap`
- instance IDs `[inst-sg-1, inst-hn-1]`
- placement rules
- gateway and domain policy

The repo alone still cannot deploy anything.

## 7. Instances, Meshes, and Clusters

### 7.1 Target resources

- `Instance`: standalone server or one server inside a mesh
- `MeshNetwork`: logical group of instances connected through overlay networking
- `Cluster`: K3s target when Kubernetes mode is enabled
- `DeploymentBinding`: logical project-to-target mapping

### 7.2 Registration without stored SSH

Preferred flow:

1. User creates an instance record in LazyOps.
2. Backend issues a short-lived bootstrap token.
3. User runs a copy-paste install command on the server.
4. Agent enrolls and exchanges bootstrap token for an agent token.
5. Backend invalidates the bootstrap token.

Optional flow:

- one-time SSH bootstrap can exist later, but credentials must never be retained.

## 8. Build and Revision Pipeline

### 8.1 Build inputs

- GitHub repository
- commit SHA
- `lazyops.yaml`
- branch and event metadata

### 8.2 Build outputs

- image reference
- image digest
- build logs
- detected services and build metadata
- artifact bundle for runtime planning

### 8.3 Desired state pipeline

1. Parse `lazyops.yaml`.
2. Validate project ownership and binding ownership.
3. Resolve `target_ref`.
4. Merge runtime defaults and environment policy.
5. Compile `Blueprint`.
6. Create `DesiredStateRevision`.
7. Dispatch to proper runtime driver.

## 9. Public API Plan

### 9.1 Auth and identity

- `POST /api/v1/auth/register`
- `POST /api/v1/auth/login`
- `POST /api/v1/auth/cli-login`
- `POST /api/v1/auth/pat/revoke`
- `GET /api/v1/auth/oauth/google/start`
- `GET /api/v1/auth/oauth/google/callback`
- `GET /api/v1/auth/oauth/github/start`
- `GET /api/v1/auth/oauth/github/callback`

### 9.2 GitHub

- `POST /api/v1/github/app/installations/sync`
- `GET /api/v1/github/repos`
- `POST /api/v1/projects/:id/repo-link`
- `POST /api/v1/integrations/github/webhook`

### 9.3 Target management

- `POST /api/v1/instances`
- `GET /api/v1/instances`
- `POST /api/v1/mesh-networks`
- `GET /api/v1/mesh-networks`
- `POST /api/v1/clusters`
- `GET /api/v1/clusters`
- `POST /api/v1/projects/:id/deployment-bindings`

### 9.4 Deploy and runtime

- `PUT /api/v1/projects/:id/blueprint`
- `POST /api/v1/projects/:id/deployments`
- `GET /api/v1/projects/:id/topology`
- `GET /api/v1/traces/:correlation_id`
- `POST /api/v1/tunnels/db/sessions`

## 10. Gateway, Domains, and Traffic Policy

### 10.1 Magic domain defaults

- primary: `sslip.io`
- fallback: `nip.io`

### 10.2 URL patterns

- `https://web.<public-ip>.sslip.io`
- `https://api.<public-ip>.sslip.io`
- `https://web.<public-ip>.nip.io` as fallback

### 10.3 Gateway responsibilities

`Caddy Gateway` must support:

- automatic HTTPS
- public route registration
- zero-downtime traffic shifting
- request holding for scale-to-zero wake-up
- `X-Correlation-ID` injection

## 11. Reliability

### 11.1 Zero-downtime

1. Create candidate revision.
2. Start candidate workload.
3. Run health checks.
4. Shift traffic.
5. Drain previous revision.

### 11.2 Global rollback

If the new revision enters crash loop or unhealthy state after promotion:

1. incident is emitted
2. backend marks the revision failed
3. desired state is reverted to the previous stable revision
4. drivers or gateway shift traffic back

### 11.3 Scale-to-zero

Support matrix:

- `standalone`: yes
- `distributed-mesh`: yes
- `distributed-k3s`: staged after baseline rollout support

## 12. Observability and FinOps

### 12.1 Observability

Backend stores and serves:

- deployment logs
- runtime incidents
- trace summaries
- node latency reports
- topology graph data

### 12.2 FinOps

Backend must ingest only aggregated edge windows:

- `p95`
- `max`
- `min`
- `avg`
- `count`

## 13. Parallel Work Plan

### 13.1 Backend tracks

- auth and identity
- GitHub App integration
- instance and binding APIs
- blueprint compiler
- build orchestration
- runtime driver contracts
- topology, traces, metrics, and incidents

### 13.2 Contracts to publish early

- auth routes
- GitHub install sync routes
- instance, mesh, cluster, and binding routes
- webhook normalized payload schema
- blueprint schema
- topology graph schema
- trace summary schema

### 13.3 Backend blockers to avoid

- do not couple frontend progress to real rollout implementation
- do not couple CLI `init` to final frontend screens
- do not let GitHub App work wait for runtime driver completion
