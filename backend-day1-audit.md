# Backend Day 1 Baseline Audit

## Summary

Day 1 xác nhận backend hiện tại mới là baseline rất sớm. Hệ thống đã có khung server Go với `Gin`, `GORM`, auth JWT cơ bản, `User` và `Agent`, middleware nền, và một WebSocket hub đơn giản. Tuy nhiên backend vẫn chưa chạm tới phần lớn product contract được yêu cầu bởi `backend-guide.md`, `lazyops-implementation-master-plan.md`, và `lazyops-master-des.md`.

Kết luận Day 1:

- Có thể tái sử dụng phần `auth`, middleware, DB bootstrap và WebSocket hub làm hạ tầng nền.
- Cần refactor từ package `service` hiện tại sang module layout theo spec thay vì tiếp tục mở rộng ad-hoc.
- `GitHub App`, `Project`, `DeploymentBinding`, build/revision pipeline, runtime driver abstraction, topology/tracing/metrics/incidents và tunnel APIs hiện đều chưa tồn tại.
- Backend hiện mới có `X-Request-ID`; spec yêu cầu `X-Correlation-ID` cho mọi public request, nên đây là gap chức năng chứ chưa thể coi là hoàn thành.
- Không có `*_test.go` trong `backend/`, nên Day 4 trở đi phải xem test là deliverable bắt buộc, không phải việc làm sau.

## Source Documents

- `guide/backend-guide.md`
- `guide/lazyops-implementation-master-plan.md`
- `guide/lazyops-master-des.md`
- `guide/project-rules.md`
- `backend-task.md`

## Current Backend Inventory

### Entrypoint and bootstrap

- `backend/cmd/server/main.go`: load config, setup logger, bootstrap app, start Gin router.
- `backend/internal/bootstrap/app.go`: wire `DB`, `Hub`, `AuthService`, `UserService`, `AgentService`, và một `GeminiClient` placeholder.
- `backend/internal/bootstrap/database.go`: connect PostgreSQL qua `GORM`, auto-migrate duy nhất `User` và `Agent`.
- `backend/internal/bootstrap/seed.go`: seed admin account cho môi trường hiện tại.

### Current HTTP and WebSocket surfaces

Routes hiện có:

- `GET /api/v1/health`
- `POST /api/v1/auth/register`
- `POST /api/v1/auth/login`
- `GET /api/v1/users/me`
- `GET /api/v1/agents`
- `POST /api/v1/agents`
- `PUT /api/v1/agents/:agentID/status`
- `GET /api/v1/ws/agents`

Nhận xét:

- Chỉ có email/password auth cơ bản; chưa có `PAT`, Google OAuth2, GitHub OAuth2, hay agent enrollment token flow.
- WebSocket hiện là stream agent status sau auth user; chưa phải `GET /ws/operators/stream`, `GET /ws/agents/control`, hoặc `GET /ws/logs/stream` theo spec.
- Chưa có nhóm routes cho `GitHub`, `projects`, `instances`, `mesh-networks`, `clusters`, `deployment-bindings`, `deployments`, `topology`, `traces`, `tunnels`.

### Middleware and cross-cutting pieces

Middleware hiện có:

- `RequestID`
- `Recovery`
- `RequestLogger`
- `SecurityHeaders`
- `CORS`
- `Timeout`
- `RateLimit`
- `Authenticate`
- `RequireRoles`

Nhận xét:

- `RequestID` đang dùng header `X-Request-ID`, không phải `X-Correlation-ID`.
- Có role guard cơ bản `admin`, `operator`, `viewer`, nhưng chưa có ownership guard theo `project`, `target`, `repo link`, `binding`.
- Có CORS, timeout và rate limit nền tảng tốt để tái sử dụng cho API surface lớn hơn.

### Current persistence and domain

Models hiện có:

- `User`
- `Agent`

Repositories hiện có:

- `UserRepository`
- `AgentRepository`

Services hiện có:

- `AuthService`
- `UserService`
- `AgentService`

Domain hiện thiếu hoàn toàn:

- `OAuthIdentity`
- `PersonalAccessToken`
- `GitHubInstallation`
- `Instance`
- `MeshNetwork`
- `Cluster`
- `DeploymentBinding`
- `Project`
- `ProjectRepoLink`
- `Service`
- `Blueprint`
- `DesiredStateRevision`
- `Deployment`
- `PreviewEnvironment`
- `TraceSummary`
- `NodeLatencyReport`
- `MetricRollup`
- `RuntimeIncident`
- `TopologyNode`
- `TopologyEdge`

### Reusable baseline pieces

- `AuthService` hiện xử lý `register/login` với JWT, có thể tách vào `internal/auth`.
- `Authenticate` và `RequireRoles` có thể thành nền cho auth/permission stack rộng hơn.
- `Hub` và `WebSocketController` có thể được tái dùng cho operator stream và control channel, nhưng cần tách contract/event rõ ràng.
- DB bootstrap và config loading hiện đã đủ làm nền cho migration và module wiring lớn hơn.

### Out-of-scope or low-priority placeholders

- `backend/internal/ai/gemini.go` chỉ là placeholder client với `APIKey` và chưa nằm trong execution spec của backend v1. Module này không được phép chặn workstream control plane chính.

## Target Module Map

| Target module | Responsibility | Current baseline | Day 1 decision |
| --- | --- | --- | --- |
| `internal/auth` | email/password login, session JWT, permission entrypoints | partial trong `service/auth_service.go` + auth controller + middleware | tách từ code hiện tại và mở rộng thay vì giữ logic auth trong `service` package chung |
| `internal/oauth` | Google OAuth2, GitHub OAuth2, callback/state handling | missing | tạo module mới, không ghép vào `auth_service.go` hiện tại |
| `internal/pat` | CLI PAT issue, hash, revoke, last-used | missing | tạo module mới với storage riêng |
| `internal/github` | GitHub App sync, repo discovery, webhook verify/normalize | missing | tạo module mới, là blocker sớm sau auth |
| `internal/projects` | project CRUD, ownership, repo link coordination | missing | tạo module mới |
| `internal/services` | product-level `Service` metadata, healthcheck, runtime profile | missing | không dùng package `service` hiện tại để tránh nhầm với service layer |
| `internal/instances` | instance CRUD, bootstrap token issue, enrollment mapping | missing | tạo module mới; `Agent` hiện tại không thay thế được `Instance` |
| `internal/mesh` | mesh networks, overlay policy, dependency routing inputs | missing | tạo module mới |
| `internal/clusters` | K3s cluster CRUD, validation, readiness | missing | tạo module mới |
| `internal/bindings` | `DeploymentBinding`, `target_ref` resolution | missing | tạo module mới; đây là source of truth cho deploy authority |
| `internal/blueprint` | compile logical contract thành runtime document | missing | tạo module mới |
| `internal/deployments` | revision records, rollout state, rollback records | missing | tạo module mới |
| `internal/build` | build jobs, callbacks, artifact reconciliation | missing | tạo module mới |
| `internal/runtime` | `RuntimeDriver` registry, dispatch, rollout plan orchestration | missing | tạo module mới, giữ boundary rõ cho `standalone`, `distributed-mesh`, `distributed-k3s` |
| `internal/topology` | topology graph read/write models | missing | tạo module mới |
| `internal/tracing` | trace summaries, correlation query | missing | tạo module mới |
| `internal/metrics` | metric rollup ingest, FinOps aggregates | missing | tạo module mới |
| `internal/incidents` | incident records, severity, operator drilldown | missing | tạo module mới |
| `internal/tunnels` | operator debug tunnel sessions | missing | tạo module mới |

Quyết định cấu trúc:

- Giữ `internal/api`, `internal/bootstrap`, `internal/config`, `internal/hub`, `pkg/logger`, `pkg/utils` làm lớp shared infrastructure.
- Refactor package `internal/service` hiện tại thành service layer nằm dưới từng domain module thay vì tiếp tục gom mọi business logic vào một package chung tên `service`.
- `internal/models` hiện tại chỉ là baseline bootstrap; khi domain lớn hơn cần tổ chức model theo domain hoặc ít nhất theo bounded context để tránh file dồn cục.

## Gap List vs Definition of Done

| Definition of done item | Current state | Gap summary |
| --- | --- | --- |
| 1. Login bằng email/password, Google OAuth2, GitHub OAuth2 | partial | mới có email/password; chưa có OAuth2 |
| 2. Register server không lưu long-lived SSH | not started | chưa có `Instance`, bootstrap token hay enroll flow |
| 3. Connect GitHub repo qua GitHub App | not started | chưa có GitHub App sync, repo discovery, repo link, webhook verify |
| 4. `lazyops init` tạo `lazyops.yaml` hợp lệ | not started | backend chưa publish target/binding/project contracts phục vụ CLI |
| 5. `git push` trigger Nixpacks build và deploy | not started | chưa có webhook normalize, build jobs, callback, blueprint, revision, runtime dispatch |
| 6. `pull_request` tạo preview environment | not started | chưa có preview model, lifecycle, cleanup flow |
| 7. `localhost` rescue hoặc env injection | not started | chưa có dependency binding, compatibility policy, sidecar contract |
| 8. Distributed mode kết nối service qua mesh VPN | not started | chưa có `MeshNetwork`, overlay policy, mesh-aware planner |
| 9. Public URLs qua `sslip.io` hoặc `nip.io` và HTTPS Caddy | not started | chưa có gateway/domain policy, route config, release detail APIs |
| 10. Logs, traces, topology, incidents, metrics hiển thị trên UI và CLI | not started | mới có WebSocket agent status đơn giản; chưa có observability domain |

Gaps nền tảng bổ sung:

- Chưa có `*_test.go` trong `backend/`.
- Chưa có ownership boundary cho `project`, `target`, `repo`, `binding`.
- Chưa có runtime boundary guard cho `distributed-k3s`.
- Chưa có event schema cho operator stream hoặc agent control channel.

## First Batch Contracts to Publish Early

Day 1 khóa batch contract đầu tiên để các team khác làm song song:

### 1. Auth contract v0

Consumers:

- frontend
- CLI

Minimum scope:

- `POST /api/v1/auth/register`
- `POST /api/v1/auth/login`
- `POST /api/v1/auth/cli-login`
- `POST /api/v1/auth/pat/revoke`
- `GET /api/v1/auth/oauth/google/start`
- `GET /api/v1/auth/oauth/google/callback`
- `GET /api/v1/auth/oauth/github/start`
- `GET /api/v1/auth/oauth/github/callback`

Minimum stable fields:

- success envelope
- error envelope
- user profile shape
- token metadata shape
- OAuth start redirect behavior
- OAuth callback success/failure behavior

### 2. Target summary schema v0

Consumers:

- CLI `init`
- frontend onboarding

Minimum stable fields:

- `id`
- `name`
- `kind`
- `runtime_modes`
- `status`
- `labels`
- `public_ip`
- `agent_id`
- `provider`

### 3. Deployment binding schema v0

Consumers:

- CLI `init`
- frontend project settings
- backend rollout pipeline

Minimum stable fields:

- `id`
- `project_id`
- `name`
- `target_ref`
- `runtime_mode`
- `target_kind`
- `target_id`
- `placement_policy`
- `domain_policy`
- `compatibility_policy`
- `scale_to_zero_policy`

### 4. Webhook normalized payload schema v0

Consumers:

- build workers
- deployment orchestration
- preview lifecycle

Minimum stable fields:

- `event_kind`
- `installation_id`
- `repo_id`
- `repo_owner`
- `repo_name`
- `branch`
- `commit_sha`
- `project_id`
- `pr_number`
- `action`
- `tracked_branch_match`

### 5. Blueprint schema v0

Consumers:

- agent
- frontend deployment detail mocks
- runtime planner

Minimum stable fields:

- `project_id`
- `runtime_mode`
- `binding`
- `services`
- `dependency_bindings`
- `compatibility_policy`
- `magic_domain_policy`
- `scale_to_zero_policy`
- `artifact_metadata`

### 6. Topology graph schema v0

Consumers:

- frontend React Flow
- observability views

Minimum stable fields:

- `nodes[]`
- `edges[]`
- node status
- edge type
- latency summary
- project scoping

### 7. Trace summary schema v0

Consumers:

- frontend trace view
- CLI trace command

Minimum stable fields:

- `project_id`
- `correlation_id`
- `started_at`
- `ended_at`
- `hops`
- `slowest_hop`
- `failure_hop`
- `status`

## Runtime Boundary Decisions

Day 1 khóa các boundary sau để tránh trượt kiến trúc:

- Backend là `control plane`, không phải nơi build trong repo người dùng, không phải SSH deploy executor, và không phải hidden runtime scheduler.
- `DeploymentBinding` là nơi resolve `target_ref`; repo và `lazyops.yaml` không mang deploy authority thật.
- `GitHub App` là default cho repo access, webhook ownership và clone. GitHub OAuth2 chỉ là identity hoặc optional repo discovery support.
- `standalone` và `distributed-mesh` được phép dùng local runtime driver phía sau, nhưng public contract luôn là `service`, không lộ `container` hay machine commands ra UX chính.
- `distributed-k3s` chỉ được bật khi user chọn mode này; backend không được direct schedule workload ngoài K3s.
- Sidecar là core runtime feature cho `env injection`, `managed credential injection`, `localhost rescue`; backend phải chuẩn bị contract cho nó nhưng không được đẩy scope sidecar vào Day 1.
- Public ingress phải đi qua `Caddy Gateway`, magic domain ưu tiên `sslip.io`, fallback `nip.io`.
- Metrics phải là edge-downsampled rollups; observability contracts phải xoay quanh summary, không dựa vào full-fidelity raw capture duy nhất.
- `X-Request-ID` hiện tại chỉ là nền tảng nội bộ; không được xem là hoàn thành yêu cầu `X-Correlation-ID`.

## Immediate Inputs for Day 2

- Dùng module map ở trên làm target structure khi bắt đầu schema và migration design.
- Dùng gap list để ưu tiên `auth -> GitHub -> target enrollment -> bindings -> build -> runtime`.
- Dùng first batch contracts làm danh sách publish sớm, tránh để frontend, CLI, agent chờ implementation cuối.
- Giữ `Hub`, middleware, config và DB bootstrap làm shared infrastructure; không mở rộng business domain mới trực tiếp trong package `service` hiện tại.
