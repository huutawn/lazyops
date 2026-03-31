# Backend 30-Day Task Tracker

## Rule Lock

Tài liệu nguồn bắt buộc cho mọi task trong file này:

- `guide/backend-guide.md`
- `guide/lazyops-implementation-master-plan.md`
- `guide/lazyops-master-des.md`

Nguyên tắc khóa cứng, không được vi phạm trong bất kỳ task nào:

- Backend là `control plane` trung tâm, sở hữu auth, repo integration, desired state compilation, deployment orchestration, observability aggregation và FinOps rollups. Không build trong repo người dùng và không dựa vào stored SSH cho normal deployments.
- `Zero-inbound` là posture mặc định. Agent luôn kết nối outbound về backend.
- Không lưu `long-lived SSH private keys` cho vận hành thường xuyên. Bootstrap chỉ dùng `short-lived bootstrap token`, one-time install command, hoặc one-time SSH bootstrap không giữ credential lâu dài.
- `lazyops.yaml` là deploy contract của project nhưng chỉ chứa `logical intent`; tuyệt đối không chứa SSH credentials, private keys, raw kubeconfig, server passwords, hay long-lived GitHub credentials.
- Mapping từ repo sang target thật phải đi qua backend-managed `DeploymentBinding`; backend là nơi resolve `target_ref`.
- Runtime modes bắt buộc là `standalone`, `distributed-mesh`, `distributed-k3s`. `K3s` chỉ được cài và dùng khi user chọn `distributed-k3s`.
- Trong `distributed-k3s`, Kubernetes là workload orchestrator. Backend và agent không được bypass K3s để `docker run`, `docker stop`, hoặc trực tiếp schedule user workloads.
- GitHub OAuth2 dùng cho identity; repo access, webhook ownership, clone và build mặc định phải dùng `GitHub App` với `short-lived installation tokens`.
- Không yêu cầu ghi CI files vào repo người dùng theo mặc định.
- Compatibility sidecar là tính năng lõi. Precedence bắt buộc là `env injection -> managed credential injection -> localhost rescue`.
- Khi service giao tiếp qua nhiều server, private traffic phải đi qua `WireGuard` hoặc `Tailscale`.
- Public ingress phải hỗ trợ magic domains, ưu tiên `sslip.io`, fallback `nip.io`, và mặc định terminate HTTPS qua `Caddy Gateway`.
- `Zero-downtime rollout` là policy mặc định; `global rollback` là bắt buộc khi promoted revision trở nên unhealthy.
- `scale-to-zero` là opt-in theo service hoặc environment, không được ép toàn cục.
- Mọi request public phải có `X-Correlation-ID`.
- Metrics gửi về backend phải là `edge-downsampled rollups`; không lưu raw high-volume samples cho long-term history.
- Backend phải publish contract sớm để agent, CLI, frontend làm song song; không được chặn `frontend` vào real rollout implementation, không được chặn `CLI init` vào UI completion, và không được chờ runtime driver mới làm GitHub App.

Public APIs cần khóa sớm:

- Auth and identity: `POST /api/v1/auth/register`, `POST /api/v1/auth/login`, `POST /api/v1/auth/cli-login`, `POST /api/v1/auth/pat/revoke`, `GET /api/v1/auth/oauth/google/start`, `GET /api/v1/auth/oauth/google/callback`, `GET /api/v1/auth/oauth/github/start`, `GET /api/v1/auth/oauth/github/callback`.
- GitHub and repo: `POST /api/v1/github/app/installations/sync`, `GET /api/v1/github/repos`, `POST /api/v1/projects/:id/repo-link`, `POST /api/v1/integrations/github/webhook`.
- Targets: `POST /api/v1/instances`, `GET /api/v1/instances`, `POST /api/v1/mesh-networks`, `GET /api/v1/mesh-networks`, `POST /api/v1/clusters`, `GET /api/v1/clusters`, `POST /api/v1/projects/:id/deployment-bindings`.
- Projects and runtime: `POST /api/v1/projects`, `GET /api/v1/projects`, `PUT /api/v1/projects/:id/blueprint`, `POST /api/v1/projects/:id/deployments`, `GET /api/v1/projects/:id/topology`, `GET /api/v1/traces/:correlation_id`, `POST /api/v1/tunnels/db/sessions`.
- Streaming: `GET /ws/operators/stream`, `GET /ws/agents/control`, `GET /ws/logs/stream`.

Core models cần khóa sớm:

- Identity and auth: `User`, `OAuthIdentity`, `PersonalAccessToken`, `GitHubInstallation`.
- Targets and binding: `Instance`, `MeshNetwork`, `Cluster`, `DeploymentBinding`.
- Projects and deploy: `Project`, `ProjectRepoLink`, `Service`, `Blueprint`, `DesiredStateRevision`, `Deployment`, `PreviewEnvironment`.
- Observability and FinOps: `TraceSummary`, `NodeLatencyReport`, `MetricRollup`, `RuntimeIncident`, `TopologyNode`, `TopologyEdge`.

Internal contracts cần publish sớm:

- `RuntimeDriver` interface và runtime driver registry cho `standalone`, `distributed-mesh`, `distributed-k3s`.
- Build callback payload.
- Shared command envelope.
- Agent command set tối thiểu.
- Operator event set.
- Target summary schema.
- `lazyops.yaml` validation contract.
- Blueprint schema.
- Desired revision schema.
- Topology graph schema.
- Trace summary schema.
- Metric rollup schema.

## Status Update Rules

- Mỗi task phải kết thúc bằng đúng một trạng thái: `(pending)`, `(doing)`, `(done)`, hoặc `(blocked: <lý do ngắn>)`.
- Toàn bộ task mới tạo ban đầu để `(pending)`.
- Khi bắt đầu làm một task, chỉ đổi trạng thái của chính task đó sang `(doing)`.
- Một task chỉ được đổi sang `(done)` khi đã hoàn thành code hoặc tài liệu tương ứng, đã tự kiểm tra lại với spec liên quan, và đã chạy test hoặc check phù hợp cho phần thay đổi đó.
- Nếu bị chặn bởi dependency, hạ tầng, secrets, spec gap hoặc review blocker thì đổi sang `(blocked: <lý do ngắn>)`.
- Không để quá một task ở trạng thái `(doing)` trong cùng một section ngày.
- Nếu task trước chưa xong hoặc đang `blocked`, task phụ thuộc phía sau phải giữ `(pending)`.
- Không xóa task đã ghi; chỉ cập nhật trạng thái tại chỗ để giữ lịch sử thực thi.
- Không di chuyển task sang ngày khác để làm đẹp tiến độ. Nếu carry-over, giữ task ở ngày gốc và thêm task tiếp nối ở ngày kế tiếp nếu thật sự cần.
- Khi một task publish contract hoặc schema, phải đảm bảo contract đủ ổn định để agent, CLI hoặc frontend có thể dùng mock hoặc triển khai song song.
- Nếu task đụng tới security, auth, webhook, rollout, binding resolution hoặc K3s boundary thì phải đối chiếu lại `Rule Lock` trước khi đổi sang `(done)`.

Ví dụ cập nhật:

- `- Hoàn thiện webhook signature verification cho GitHub App. (doing)`
- `- Hoàn thiện webhook signature verification cho GitHub App. (done)`
- `- Cấu hình GitHub App production secret. (blocked: chờ secret từ hạ tầng)`

## Day 1 - Baseline Audit

Mục tiêu: đối chiếu backend hiện tại với spec, xác nhận baseline còn thiếu gì và khóa danh sách contract publish sớm.

Deliverable: `backend-day1-audit.md`.

- Audit code hiện tại của backend, liệt kê các module, routes, models, services đang có và các domain còn thiếu so với `backend-guide.md` và master spec. (done)
- Chốt target module map cho backend theo các nhóm `auth`, `oauth`, `pat`, `github`, `projects`, `services`, `instances`, `mesh`, `clusters`, `bindings`, `blueprint`, `deployments`, `build`, `runtime`, `topology`, `tracing`, `metrics`, `incidents`, `tunnels`. (done)
- Lập gap list giữa baseline hiện tại và `Definition of done` của toàn hệ thống để làm input cho backlog 30 ngày. (done)
- Khóa first batch contracts cần publish sớm cho agent, CLI và frontend: auth routes, target summary schema, deployment binding schema, webhook normalized payload, blueprint schema, topology schema, trace summary schema. (done)
- Ghi rõ runtime boundary để mọi task sau không vô tình kéo hệ thống về `K3s-first` hoặc đưa SSH quay lại flow chính. (done)

## Day 2 - Persistence Foundation

Mục tiêu: chốt domain model, migration order và các chuẩn chung cho persistence.

Deliverable: `backend-day2-persistence-foundation.md`.

- Thiết kế ID strategy, status enums, timestamp conventions và error envelope thống nhất cho toàn bộ backend. (done)
- Chốt migration order cho các domain `identity/auth`, `targets`, `projects/deploy`, `observability/FinOps` để tránh vòng phụ thuộc. (done)
- Thiết kế schema chi tiết cho `OAuthIdentity`, `PersonalAccessToken`, `GitHubInstallation`, `Project`, `ProjectRepoLink`, `Instance`, `MeshNetwork`, `Cluster`, `DeploymentBinding`, `Service`, `Blueprint`, `DesiredStateRevision`, `Deployment`, `PreviewEnvironment`, `TraceSummary`, `NodeLatencyReport`, `MetricRollup`, `RuntimeIncident`, `TopologyNode`, `TopologyEdge`. (done)
- Chuẩn hóa nguyên tắc lưu secret/token theo dạng hash hoặc encrypted-at-rest, tuyệt đối không log plaintext. (done)
- Publish persistence conventions và domain relationship notes để agent, CLI, frontend biết các ID và status nào là stable contracts. (done)

## Day 3 - API Contract Foundation

Mục tiêu: khóa response contract, auth middleware contract và permission model cho các team phụ thuộc.

Deliverable: `backend-day3-api-contract-foundation.md`.

- Chuẩn hóa response envelope cho success, validation error, auth error, permission error và system error. (done)
- Chốt auth middleware contract cho web session, bearer token và internal agent-scoped auth. (done)
- Thiết kế permission guard matrix cho `viewer`, `operator`, `admin`, project ownership và target ownership. (done)
- Publish auth request/response schema, error codes và permission semantics cho frontend và CLI. (done)
- Thêm checklist kiểm tra mọi API mới phải dùng envelope và permission guard thống nhất trước khi merge. (done)

## Day 4 - Email/Password Auth

Mục tiêu: hoàn thiện web login nền tảng đúng security rule.

- Hoàn thiện `POST /api/v1/auth/register` với validation, email normalization, password policy và account status mặc định. (pending)
- Hoàn thiện `POST /api/v1/auth/login` với credential validation, disabled or revoked account checks và last-login update. (pending)
- Tách rõ web session JWT lifecycle, expiry, issuer, claims và audit fields theo chuẩn bảo mật của backend. (pending)
- Thêm ownership guard cho các flow dùng authenticated user context để tránh cross-tenant hoặc cross-user access. (pending)
- Viết test cases cho wrong password, invalid input, disabled account và user A không thấy tài nguyên của user B. (pending)

## Day 5 - PAT for CLI

Mục tiêu: cung cấp session continuity cho CLI mà vẫn đúng security boundary.

- Thêm `PersonalAccessToken` model, hashing strategy, expiry policy, revoke state và `last_used_at`. (pending)
- Implement `POST /api/v1/auth/cli-login` để issue PAT sau khi user authenticate thành công. (pending)
- Implement `POST /api/v1/auth/pat/revoke` với ownership validation và revoke semantics rõ ràng. (pending)
- Publish PAT response schema, revoke contract và CLI storage expectations để CLI team dùng ngay. (pending)
- Viết test cho PAT issue, revoked PAT no longer works, token ownership mismatch và rate-limit abuse case. (pending)

## Day 6 - Google OAuth2

Mục tiêu: hoàn thiện login bằng Google cho web identity.

- Implement `GET /api/v1/auth/oauth/google/start` với state creation, redirect composition và anti-CSRF handling. (pending)
- Implement `GET /api/v1/auth/oauth/google/callback` với state validation, user find-or-create, `OAuthIdentity` upsert và web session issuance. (pending)
- Chuẩn hóa flow merge identity khi email đã tồn tại nhưng chưa link Google, tránh duplicate account. (pending)
- Publish callback success/failure contract để frontend xử lý redirect, retry và error display. (pending)
- Viết test cho Google OAuth success, state mismatch, revoked identity và existing-email linkage. (pending)

## Day 7 - GitHub OAuth2

Mục tiêu: dùng GitHub OAuth2 cho identity nhưng vẫn giữ GitHub App là mặc định cho repo access.

- Implement `GET /api/v1/auth/oauth/github/start` với state generation và redirect flow. (pending)
- Implement `GET /api/v1/auth/oauth/github/callback` với state validation, user find-or-create, `OAuthIdentity` upsert và session issuance. (pending)
- Chốt rule rõ ràng rằng GitHub OAuth2 chỉ dùng cho identity hoặc optional repo discovery, không thay thế GitHub App cho webhook và clone. (pending)
- Publish identity linkage contract giữa GitHub OAuth account và backend `User` để tránh ownership ambiguity ở các flow sau. (pending)
- Viết test cho callback ownership mismatch, state mismatch, revoked identity và successful login. (pending)

## Day 8 - GitHub Installation Sync

Mục tiêu: đưa GitHub App vào flow backend một cách first-class.

- Thêm `GitHubInstallation` model với `github_installation_id`, `account_login`, `account_type`, `scope_json`, `installed_at`, `revoked_at`. (pending)
- Implement `POST /api/v1/github/app/installations/sync` để fetch và sync installation records về backend. (pending)
- Chuẩn hóa installation scope validation để chỉ repo trong installation hợp lệ mới được dùng cho repo link và webhook routing. (pending)
- Publish installation summary schema và revoked-state semantics cho frontend. (pending)
- Viết test cho sync success, missing GitHub identity, revoked installation và API error handling. (pending)

## Day 9 - Projects and Repo Discovery

Mục tiêu: chuẩn bị nền cho việc link repo vào project.

- Thêm `Project` model và implement `POST /api/v1/projects`, `GET /api/v1/projects` với ownership semantics rõ ràng. (pending)
- Implement `GET /api/v1/github/repos` dựa trên GitHub App installation scope thay vì long-lived user token. (pending)
- Thiết kế project summary schema để frontend, CLI và webhook resolver cùng dùng một contract. (pending)
- Thêm validation cho project slug uniqueness, branch defaults và project ownership checks. (pending)
- Viết test cho create/list projects, unauthorized access và repo discovery theo installation scope. (pending)

## Day 10 - Repo Linking

Mục tiêu: hoàn thiện quan hệ `project <-> repo`.

- Thêm `ProjectRepoLink` model với `github_installation_id`, `github_repo_id`, `repo_owner`, `repo_name`, `tracked_branch`, `preview_enabled`. (pending)
- Implement `POST /api/v1/projects/:id/repo-link` với validation project ownership, installation access và tracked branch policy. (pending)
- Thiết kế webhook routing lookup dựa trên `ProjectRepoLink` để webhook có thể resolve `project_id`, repo và branch một cách nhất quán. (pending)
- Publish repo-link request/response schema và preview-enabled semantics cho frontend. (pending)
- Viết test cho repo link success, inaccessible repo, invalid branch và ownership mismatch. (pending)

## Day 11 - Webhook Trust and Normalize

Mục tiêu: xử lý webhook GitHub App đúng trust model và tạo event normalized cho build plane.

- Implement `POST /api/v1/integrations/github/webhook` với signature verification và reject mọi payload invalid. (pending)
- Normalize các sự kiện `push`, `pull_request`, `pull_request.closed` thành payload backend thống nhất cho build and preview flows. (pending)
- Publish normalized webhook payload schema và build trigger contract cho build plane. (pending)
- Chuẩn hóa audit/logging để không rò secret nhưng vẫn trace được installation, project, branch, commit và event type. (pending)
- Viết test cho invalid signature, unlinked repo ignored, push trigger, PR open trigger và PR close trigger. (pending)

## Day 12 - Instance CRUD

Mục tiêu: bắt đầu target enrollment không cần lưu SSH.

- Thêm `Instance` model và implement `POST /api/v1/instances`, `GET /api/v1/instances`. (pending)
- Thiết kế bootstrap token lifecycle cho instance enrollment, gồm issue, expiry, single-use semantics và ownership binding. (pending)
- Chuẩn hóa `Instance` status model, capability summary và labels contract cho frontend/CLI target chooser. (pending)
- Publish target summary schema cho CLI `init` và frontend onboarding. (pending)
- Viết test cho duplicate instance name, invalid IP, bootstrap token issue và instance list ownership. (pending)

## Day 13 - Agent Enrollment

Mục tiêu: đổi bootstrap token sang agent session đúng zero-inbound posture.

- Implement bootstrap-token exchange flow để agent nhận `agent token` và bind vào đúng `Instance`. (pending)
- Invalidate bootstrap token ngay sau enrollment thành công và chặn reuse hoặc expired token. (pending)
- Thêm capability report, heartbeat và online status update cho agent sau enrollment. (pending)
- Publish agent enrollment contract, heartbeat payload và capability schema cho agent team. (pending)
- Viết test cho expired bootstrap token, reused token, ownership mismatch và instance online after enrollment. (pending)

## Day 14 - Mesh and Cluster CRUD

Mục tiêu: đưa `distributed-mesh` và `distributed-k3s` target models vào backend.

- Thêm `MeshNetwork` model và implement `POST /api/v1/mesh-networks`, `GET /api/v1/mesh-networks`. (pending)
- Thêm `Cluster` model và implement `POST /api/v1/clusters`, `GET /api/v1/clusters`. (pending)
- Thiết kế validation để `MeshNetwork` chỉ nhận provider hợp lệ như `wireguard` hoặc `tailscale`, và `Cluster` chỉ nhận `k3s`. (pending)
- Publish target-kind summary contract và mode compatibility rules cho CLI `init` và frontend target UI. (pending)
- Viết test cho create/list mesh networks, create/list clusters, invalid provider và ownership enforcement. (pending)

## Day 15 - Deployment Binding

Mục tiêu: chốt logical deploy authority qua `DeploymentBinding`.

- Thêm `DeploymentBinding` model với `target_ref`, `runtime_mode`, `target_kind`, `target_id`, `placement_policy_json`, `domain_policy_json`, `compatibility_policy_json`, `scale_to_zero_policy_json`. (pending)
- Implement `POST /api/v1/projects/:id/deployment-bindings` với validation project ownership, target ownership, `target_ref` uniqueness và mode compatibility. (pending)
- Chốt rule backend-only resolution của `target_ref` để repo không bao giờ tự chứa deploy authority. (pending)
- Publish deployment binding schema và target selection contract cho CLI `init`. (pending)
- Viết test cho duplicate `target_ref`, unknown target, mismatched runtime mode và successful binding creation. (pending)

## Day 16 - Init-Facing Backend Contracts

Mục tiêu: cho phép `lazyops init` dựa vào backend contract ổn định.

- Publish APIs và data shape mà `lazyops init` cần dùng: projects, instances, mesh networks, clusters, deployment bindings và target summaries. (pending)
- Thiết kế validation contract cho `lazyops.yaml` để chỉ chấp nhận `logical intent` và reject secret-bearing payloads hoặc hard-coded deploy authority. (pending)
- Chốt dependency binding validation rules cho service name, alias, protocol, `target_service` và `local_endpoint`. (pending)
- Publish `lazyops.yaml` minimum schema và error model để CLI có thể validate trước khi write file. (pending)
- Viết test cho unknown `target_ref`, invalid dependency mapping và secret-bearing config bị reject. (pending)

## Day 17 - Blueprint Baseline

Mục tiêu: bắt đầu compile deploy contract thành runtime document nội bộ.

- Thêm `Service` model để lưu service metadata, path, public flag, runtime profile và `healthcheck_json`. (pending)
- Thêm `Blueprint` model và định nghĩa compile input từ `lazyops.yaml`, project state, repo link và binding state. (pending)
- Chốt blueprint schema để agent, frontend và các runtime planners có thể làm song song. (pending)
- Publish desired revision draft contract để frontend mock deployment details và agent hiểu payload shape sẽ nhận sau này. (pending)
- Viết test cho blueprint compile success, invalid `lazyops.yaml` và missing binding rejection. (pending)

## Day 18 - Revision and Deployment Records

Mục tiêu: ghi nhận desired state và deployment lifecycle trong backend.

- Thêm `DesiredStateRevision` model với `commit_sha`, `trigger_kind`, `status`, `compiled_revision_json`. (pending)
- Thêm `Deployment` model với rollout lifecycle fields và status machine rõ ràng. (pending)
- Implement manual deploy entrypoint qua `POST /api/v1/projects/:id/deployments` với ownership validation và revision creation flow. (pending)
- Chuẩn hóa revision status machine cho `queued`, `building`, `planned`, `applying`, `promoted`, `failed`, `rolled_back` hoặc các trạng thái tương đương đã khóa. (pending)
- Viết test cho manual deploy success, ownership mismatch, invalid revision state transition và deployment record creation. (pending)

## Day 19 - Build Orchestration

Mục tiêu: biến normalized webhook event thành build job thật.

- Thiết kế build job queue/orchestration layer cho các commit đến từ webhook. (pending)
- Enforce branch policy dựa trên `ProjectRepoLink` trước khi enqueue build job. (pending)
- Chuẩn hóa build job record, artifact metadata staging và retry policy cho build plane. (pending)
- Publish build job contract, worker input schema và callback expectations cho build worker track. (pending)
- Viết test cho webhook-to-build dispatch, branch rejected case và build job persistence. (pending)

## Day 20 - Build Callback and Artifact Reconciliation

Mục tiêu: nhận kết quả build và nối vào revision pipeline.

- Implement build callback endpoint theo payload chuẩn gồm `build_job_id`, `project_id`, `commit_sha`, `status`, `image_ref`, `image_digest`, `metadata.detected_services`. (pending)
- Reconcile artifact metadata vào `Blueprint` hoặc `DesiredStateRevision` pipeline và persist detected services khi phù hợp. (pending)
- Chuyển build failure thành operator-visible event để frontend có thể hiển thị deployment/build failure rõ ràng. (pending)
- Publish finalized build callback schema và artifact metadata contract cho build plane và frontend. (pending)
- Viết test cho callback success, artifact mismatch, unknown build job và build failure propagation. (pending)

## Day 21 - Runtime Driver Abstraction

Mục tiêu: dựng xương sống runtime độc lập target mode.

- Implement `RuntimeDriver` registry cho `standalone`, `distributed-mesh`, `distributed-k3s`. (pending)
- Chuẩn hóa shared command envelope với `type`, `request_id`, `correlation_id`, `agent_id`, `project_id`, `source`, `occurred_at`, `payload`. (pending)
- Thiết kế validation boundary để `distributed-k3s` chỉ plan qua Kubernetes-facing driver và không cho direct workload scheduling ngoài K3s. (pending)
- Publish runtime driver contracts và command envelope cho agent team. (pending)
- Viết test cho driver resolution theo runtime mode, invalid target validation và K3s boundary guard. (pending)

## Day 22 - Control and Operator Streams

Mục tiêu: mở control channel và operator event stream đúng contract.

- Implement `GET /ws/agents/control` cho agent control channel. (pending)
- Implement `GET /ws/operators/stream` cho operator event stream. (pending)
- Map minimum agent command set: `reconcile_revision`, `prepare_release_workspace`, `ensure_mesh_peer`, `sync_overlay_routes`, `render_sidecars`, `render_gateway_config`, `start_release_candidate`, `run_health_gate`, `promote_release`, `rollback_release`, `wake_service`, `sleep_service`, `report_topology_state`, `report_trace_summary`, `report_metric_rollup`, `garbage_collect_runtime`. (pending)
- Publish operator event set gồm `deployment.started`, `deployment.build_failed`, `deployment.candidate_ready`, `deployment.promoted`, `deployment.rolled_back`, `incident.created`, `trace.recorded`, `topology.updated`, `metric.rollup_ingested`. (pending)
- Viết test cho stream auth, event delivery, control channel registration và malformed event rejection. (pending)

## Day 23 - Standalone Rollout

Mục tiêu: hoàn thiện baseline rollout cho `standalone`.

- Implement planner để tạo candidate rollout từ `DesiredStateRevision` tới `standalone` runtime driver. (pending)
- Implement health gate, candidate readiness check và promotion flow theo zero-downtime default policy. (pending)
- Implement rollback path về last stable revision khi candidate fail trước hoặc sau promotion. (pending)
- Tạo `RuntimeIncident` records cho unhealthy candidate, crash loop hoặc promotion failure. (pending)
- Viết test cho push-to-deployment baseline, health failure blocks promotion, rollback restores stable revision và incident creation. (pending)

## Day 24 - Gateway and Domains

Mục tiêu: đưa public URL, domain policy và release visibility vào backend.

- Thiết kế backend model cho public route, gateway config intent và release history records. (pending)
- Implement magic domain policy ưu tiên `sslip.io`, fallback `nip.io`, và reject public IP không hợp lệ cho magic domain. (pending)
- Publish deployment history API và release detail API cho frontend. (pending)
- Chuẩn hóa `Caddy Gateway` handoff contract cho route registration, HTTPS provisioning, traffic shifting và wake-up hold semantics. (pending)
- Viết test cho domain allocation success, invalid public IP, release history query và gateway config payload generation. (pending)

## Day 25 - Preview Environments

Mục tiêu: hỗ trợ `pull_request` preview lifecycle end-to-end từ góc nhìn backend.

- Thêm `PreviewEnvironment` model với `pr_number`, `status`, `domain_json`, `destroyed_at`. (pending)
- Implement flow tạo preview revision khi nhận `pull_request` event hợp lệ và project bật `preview_enabled`. (pending)
- Implement flow cleanup preview khi nhận `pull_request.closed`, kể cả khi target đang degraded hoặc partially unavailable. (pending)
- Publish preview URL contract và preview status model cho frontend và GitHub integration track. (pending)
- Viết test cho PR open creates preview, PR close destroys preview, capacity unavailable case và cleanup idempotency. (pending)

## Day 26 - Mesh Backend Planning

Mục tiêu: chuẩn bị backend cho `distributed-mesh` mà không ép Kubernetes.

- Implement dependency binding resolution để backend quyết định service reachability qua env injection hoặc proxy route cho `distributed-mesh`. (pending)
- Thiết kế private-path policy buộc cross-node service traffic đi qua `WireGuard` hoặc `Tailscale`. (pending)
- Implement `POST /api/v1/tunnels/db/sessions` cho secure operator tunnel tới internal service. (pending)
- Thêm topology state ingest contract cho mesh nodes và publish schema cho agent team. (pending)
- Viết test cho dependency resolution success, unsupported protocol, target offline tunnel và cross-node private-path enforcement. (pending)

## Day 27 - K3s Backend Track

Mục tiêu: hỗ trợ `distributed-k3s` như advanced mode có boundary rõ ràng.

- Hoàn thiện `Cluster` validation, kube access checks và target readiness model cho `distributed-k3s`. (pending)
- Thiết kế `distributed-k3s` runtime driver planning theo hướng backend phát desired state và policy, còn workload scheduling do K3s đảm nhiệm. (pending)
- Publish `node_agent` telemetry contract và cluster-side reporting expectations cho agent team. (pending)
- Thêm guard rõ ràng trong planner và runtime layer để backend không bypass K3s khi schedule user workloads. (pending)
- Viết test cho cluster validation success, kube API unreachable, node telemetry contract và K3s-boundary enforcement. (pending)

## Day 28 - Observability APIs

Mục tiêu: làm cho failure có thể debug được trên backend surface.

- Chốt và publish contract propagation của `X-Correlation-ID` từ gateway vào sidecar, agent và trace storage. (pending)
- Implement trace summary ingest và `GET /api/v1/traces/:correlation_id`. (pending)
- Implement incident drilldown read model và incident query surface cho frontend. (pending)
- Implement topology graph read model và `GET /api/v1/projects/:id/topology`. (pending)
- Viết test cho correlation id presence, trace query success, topology graph consistency và incident visibility. (pending)

## Day 29 - FinOps and Scale-to-Zero

Mục tiêu: tối ưu chi phí và giữ metrics đủ rẻ cho long-term storage.

- Implement metric rollup ingest cho `p95`, `max`, `min`, `avg`, `count` và reject raw high-volume samples không đúng contract. (pending)
- Thêm `MetricRollup` storage strategy cho long-window summaries theo target và service. (pending)
- Thiết kế hot-edge, hot-node và utilization summary logic để phục vụ FinOps views. (pending)
- Thêm scale-to-zero policy state cho `standalone` và `distributed-mesh`, bao gồm wake-up hold contract với gateway. (pending)
- Viết test cho aggregate-only ingest, rollup correctness, opt-in scale-to-zero và wake timeout behavior. (pending)

## Day 30 - Hardening and Acceptance Gate

Mục tiêu: khóa chất lượng v1 và hoàn tất handoff contracts cho các team khác.

- Chạy security matrix cho auth, PAT, OAuth, webhook verification, agent token, target ownership và secret logging rules. (pending)
- Chạy acceptance matrix cho target enrollment, deploy contract validation, standalone rollout, mesh routing, K3s boundary, observability và FinOps. (pending)
- Chạy rollback tests, preview cleanup tests, webhook trust tests, mesh failure tests và ownership isolation tests. (pending)
- Hoàn thiện runbooks, operational notes và contract freeze list cho agent, CLI và frontend. (pending)
- Review toàn bộ file `backend-task.md`, cập nhật trạng thái thực tế, chốt phần nào còn `blocked` và ghi rõ dependency ngoài backend nếu có. (pending)
