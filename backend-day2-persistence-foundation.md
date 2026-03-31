# Backend Day 2 Persistence Foundation

## Summary

Day 2 khóa nền tảng persistence cho backend để Day 3 trở đi có thể code mà không phải tự quyết định lại về ID, migration order, trạng thái, timestamp, hay secret handling.

Các quyết định chính:

- Chuyển toàn bộ domain backend mục tiêu sang `string primary ID` dạng `prefix + ULID` để khớp API/event examples như `prj_123`, `bld_123`, `corr_123` trong master spec.
- Dùng `PostgreSQL` làm source of truth, `timestamptz` cho mọi thời điểm, `jsonb` cho policy/config blobs, `inet/cidr` cho network fields khi phù hợp.
- Chỉ dùng `status` column cho resource có lifecycle vận hành cần query trực tiếp; các resource immutable hoặc state suy ra được sẽ dùng timestamps thay vì thêm status giả.
- `token`, `secret`, `private key`, `kubeconfig` raw không được lưu plaintext. Token so sánh một chiều phải lưu `hash`; secret cần dùng lại phải lưu bằng `secret_ref` hoặc encrypted-at-rest.
- Migrate khỏi `AutoMigrate` toàn cục trước khi mở rộng domain thật sự; từ Day 3 trở đi backend phải chuyển sang versioned migrations.

Tài liệu nguồn:

- `backend-day1-audit.md`
- `backend-task.md`
- `guide/backend-guide.md`
- `guide/lazyops-implementation-master-plan.md`
- `guide/lazyops-master-des.md`
- `guide/project-rules.md`

## Persistence Conventions

### 1. ID strategy

Chuẩn chính thức:

- Mọi resource mới dùng `id text primary key`.
- Format chuẩn là `prefix_<ulid>`.
- `ULID` được tạo tại backend, là opaque, immutable, sortable theo thời gian, không tiết lộ số lượng bản ghi.
- Tất cả API, event, WebSocket payload, build callback, trace lookup và internal command envelope đều dùng cùng một ID string này.
- Foreign keys giữa các bảng cũng dùng cùng string ID để tránh dual-ID complexity giữa DB và API.

Quy ước prefix:

| Model | Prefix |
| --- | --- |
| `User` | `usr_` |
| `OAuthIdentity` | `oid_` |
| `PersonalAccessToken` | `pat_` |
| `GitHubInstallation` | `ghi_` |
| `Agent` | `agt_` |
| `Instance` | `inst_` |
| `MeshNetwork` | `mesh_` |
| `Cluster` | `cls_` |
| `DeploymentBinding` | `bind_` |
| `Project` | `prj_` |
| `ProjectRepoLink` | `prl_` |
| `Service` | `svc_` |
| `Blueprint` | `bp_` |
| `DesiredStateRevision` | `rev_` |
| `Deployment` | `dep_` |
| `PreviewEnvironment` | `prev_` |
| `TraceSummary` | `trc_` |
| `NodeLatencyReport` | `lat_` |
| `MetricRollup` | `mtr_` |
| `RuntimeIncident` | `inc_` |
| `TopologyNode` | `tn_` |
| `TopologyEdge` | `te_` |

Support IDs dự kiến cho các phase sau:

- `BuildJob`: `bld_`
- `BootstrapToken`: `boot_`
- `AgentToken`: `atok_`
- `TunnelSession`: `tun_`
- `RequestID`: `req_`
- `CorrelationID`: `corr_`

Quyết định migration:

- `User.ID uint` và `Agent.ID uint` trong baseline hiện tại chỉ là tạm thời.
- Trước khi thêm các domain table mới, backend phải migrate `User` và `Agent` sang string IDs để giữ một chuẩn thống nhất cho toàn control plane.
- Trong API contract, field name vẫn giữ dạng `id`, `user_id`, `project_id`, `revision_id`, nhưng giá trị sẽ là string IDs có prefix.

### 2. Timestamp conventions

- Mọi bảng mutable có ít nhất `created_at timestamptz not null` và `updated_at timestamptz not null`.
- Mọi timestamp đều lưu ở `UTC`.
- API chỉ trả về RFC3339 UTC timestamps.
- Timestamps sự kiện hoặc lifecycle dùng chuẩn `*_at`, ví dụ `expires_at`, `revoked_at`, `started_at`, `completed_at`, `destroyed_at`.
- Trường nullable chỉ dùng khi sự kiện có thể chưa xảy ra.
- Không dùng local timezone trong database dù config app có timezone mặc định.
- Không dùng GORM soft delete mặc định cho domain chính; ưu tiên `revoked_at`, `destroyed_at`, `archived_at` hoặc `status` tường minh để audit dễ hơn.

### 3. Column type conventions

- IDs: `text`
- Status/severity/kind/provider/runtime mode: `text` với application validation, có thể thêm `CHECK` ở migration khi enum ổn định
- Email, slug, repo owner/name, target_ref: `text`, normalized ở app layer
- IP address: `inet`
- CIDR: `cidr`
- Policy/config/metadata payloads: `jsonb`
- Trace, topology metadata, rollups: `jsonb`
- Large secret payloads cần tái sử dụng: không lưu raw, chỉ lưu `secret_ref` hoặc encrypted blob nếu bất khả kháng

### 4. Envelope and error baseline

Day 3 sẽ khóa HTTP contract đầy đủ, nhưng Day 2 chốt semantics nền:

Success envelope tối thiểu:

```json
{
  "success": true,
  "message": "ok",
  "request_id": "req_01...",
  "correlation_id": "corr_01...",
  "data": {},
  "meta": {}
}
```

Error envelope tối thiểu:

```json
{
  "success": false,
  "message": "repo link failed",
  "request_id": "req_01...",
  "correlation_id": "corr_01...",
  "error": {
    "code": "repo_access_denied",
    "details": "installation does not include repository",
    "fields": {
      "project_id": "prj_01..."
    },
    "retryable": false
  }
}
```

Quy ước:

- `request_id` là per-request identifier.
- `correlation_id` là end-to-end flow identifier; có thể chưa xuất hiện ở mọi flow ngay Day 3 nhưng field semantics đã được khóa.
- `error.code` là machine-readable stable key.
- `message` là human-readable summary.
- `error.details` có thể là string hoặc object nhỏ, không được chứa secret.

## Status Design

Nguyên tắc:

- Chỉ resource cần filter/query trực tiếp theo lifecycle mới có `status` persisted.
- Resource mà state suy ra được từ timestamps thì không thêm `status`.
- Resource immutable như `Blueprint` không có `status`.

Persisted status enums:

| Model | Status values |
| --- | --- |
| `User` | `active`, `disabled` |
| `Instance` | `pending_enrollment`, `online`, `offline`, `degraded`, `revoked` |
| `MeshNetwork` | `provisioning`, `active`, `degraded`, `revoked` |
| `Cluster` | `validating`, `ready`, `degraded`, `unreachable`, `revoked` |
| `DesiredStateRevision` | `draft`, `queued`, `building`, `artifact_ready`, `planned`, `applying`, `promoted`, `failed`, `rolled_back`, `superseded` |
| `Deployment` | `queued`, `running`, `candidate_ready`, `promoted`, `failed`, `rolled_back`, `canceled` |
| `PreviewEnvironment` | `provisioning`, `ready`, `failed`, `destroying`, `destroyed` |
| `TopologyNode` | `healthy`, `degraded`, `unreachable`, `unknown` |
| `TopologyEdge` | `healthy`, `degraded`, `unreachable`, `unknown` |

Derived states, không cần `status` column riêng:

| Model | Derived state rule |
| --- | --- |
| `PersonalAccessToken` | `active` nếu chưa `revoked_at` và chưa quá `expires_at`; ngược lại là `revoked` hoặc `expired` |
| `GitHubInstallation` | `active` nếu chưa `revoked_at`; `revoked` nếu đã có `revoked_at` |
| `OAuthIdentity` | `linked` nếu row tồn tại và chưa `revoked_at`; `revoked` nếu có `revoked_at` |
| `ProjectRepoLink` | `active` nếu row còn hiệu lực và repo vẫn accessible |
| `TraceSummary` | `ok`, `error`, `partial` suy từ `trace_json` summary chứ không lưu column riêng |
| `MetricRollup` | không có lifecycle status; bản chất là immutable aggregate window |

Severity and kind enums:

- `RuntimeIncident.severity`: `info`, `warning`, `critical`
- `Deployment.trigger_kind`: `push`, `pull_request`, `manual`
- `Blueprint.source_kind`: `lazyops_yaml`, `ui_plan`, `api_update`
- `MeshNetwork.provider`: `wireguard`, `tailscale`
- `Cluster.provider`: `k3s`
- `DeploymentBinding.target_kind`: `instance`, `mesh`, `cluster`
- `DeploymentBinding.runtime_mode`: `standalone`, `distributed-mesh`, `distributed-k3s`

## Migration Order

Quyết định chính:

- Không tiếp tục mở rộng `bootstrap.Migrate()` theo kiểu `AutoMigrate` cho toàn bộ control plane.
- Từ Day 3 trở đi dùng versioned migrations theo thứ tự domain dưới đây.
- Ưu tiên tạo bảng cha trước, bảng con sau, tránh polymorphic FK cứng với `target_kind/target_id`.

### Phase 0 - Shared migration prerequisites

- Chuẩn hóa ID generator ở app layer.
- Chốt helper types cho `jsonb`, `inet`, `cidr`, `timestamptz`.
- Migrate `users` và `agents` khỏi numeric IDs sang string IDs.
- Chốt naming conventions cho indexes và constraints.

### Phase 1 - Identity and auth

Thứ tự:

1. `users`
2. `oauth_identities`
3. `personal_access_tokens`
4. `github_installations`

Lý do:

- `User` là root entity cho auth và ownership.
- `OAuthIdentity`, `PersonalAccessToken`, `GitHubInstallation` đều phụ thuộc `users`.

### Phase 2 - Targets

Thứ tự:

1. `instances`
2. `mesh_networks`
3. `clusters`

Ghi chú:

- `instances.agent_id` có thể nullable khi target vừa tạo nhưng agent chưa enroll.
- `target_kind/target_id` ở binding sẽ không tạo FK polymorphic ở DB layer.
- Support tables như `bootstrap_tokens` và `agent_tokens` sẽ được thêm sau core target tables, trước Day 13 implementation.

### Phase 3 - Projects and deploy contract

Thứ tự:

1. `projects`
2. `project_repo_links`
3. `deployment_bindings`
4. `services`
5. `blueprints`

Lý do:

- `Project` là root entity cho repo link, binding và revision lineage.
- `ProjectRepoLink` cần `projects` và `github_installations`.
- `DeploymentBinding` cần `projects` và target identifiers đã có từ Phase 2.
- `Service` và `Blueprint` là contract-level resources gắn với `projects`.

### Phase 4 - Revision and rollout state

Thứ tự:

1. `desired_state_revisions`
2. `deployments`
3. `preview_environments`

Lý do:

- `DesiredStateRevision` phụ thuộc `projects`, `blueprints`, `deployment_bindings`.
- `Deployment` và `PreviewEnvironment` đều phụ thuộc `desired_state_revisions`.

### Phase 5 - Observability and FinOps

Thứ tự:

1. `trace_summaries`
2. `node_latency_reports`
3. `metric_rollups`
4. `runtime_incidents`
5. `topology_nodes`
6. `topology_edges`

Lý do:

- Các bảng này đều phụ thuộc ít nhất `projects`.
- `runtime_incidents` nên tới sau `desired_state_revisions` để có thể tham chiếu `revision_id`.
- `topology_edges` phụ thuộc semantically vào `topology_nodes`, nhưng relation được giữ bằng `node_ref` thay vì hard FK để ingest linh hoạt hơn.

## Schema Details

### Identity and auth

#### `User`

Purpose:

- root identity record cho ownership toàn backend

Columns:

- `id text primary key`
- `email text not null unique`
- `password_hash text null`
- `display_name text not null`
- `status text not null default 'active'`
- `created_at timestamptz not null`
- `updated_at timestamptz not null`

Indexes and rules:

- unique index trên `email`
- app layer phải lowercase + trim email trước khi persist
- `password_hash` cho phép null để hỗ trợ OAuth-only accounts

#### `OAuthIdentity`

Purpose:

- map external identity từ `google` hoặc `github` vào `User`

Columns:

- `id text primary key`
- `user_id text not null`
- `provider text not null`
- `provider_subject text not null`
- `email text null`
- `avatar_url text null`
- `revoked_at timestamptz null`
- `created_at timestamptz not null`
- `updated_at timestamptz not null`

Indexes and rules:

- unique index trên `(provider, provider_subject)`
- index trên `user_id`
- `provider` chỉ nhận `google` hoặc `github`

Decision note:

- `revoked_at` là implementation extension cần thiết để đáp ứng failure path “revoked identity” trong spec.

#### `PersonalAccessToken`

Purpose:

- CLI session continuity

Columns:

- `id text primary key`
- `user_id text not null`
- `name text not null`
- `token_hash text not null`
- `token_prefix text not null`
- `last_used_at timestamptz null`
- `expires_at timestamptz null`
- `revoked_at timestamptz null`
- `created_at timestamptz not null`

Indexes and rules:

- unique index trên `token_hash`
- index trên `user_id`
- raw token chỉ xuất hiện một lần khi issue, không persist
- `token_prefix` chỉ dùng cho UX/audit, không dùng để authenticate

#### `GitHubInstallation`

Purpose:

- lưu installation visibility của GitHub App cho user hiện tại

Columns:

- `id text primary key`
- `user_id text not null`
- `github_installation_id bigint not null`
- `account_login text not null`
- `account_type text not null`
- `scope_json jsonb not null default '{}'`
- `installed_at timestamptz not null`
- `revoked_at timestamptz null`
- `created_at timestamptz not null`
- `updated_at timestamptz not null`

Indexes and rules:

- unique index trên `(user_id, github_installation_id)`
- index trên `account_login`
- không persist installation access token; token chỉ dùng in-memory trong sync/build flows

### Targets

#### `Instance`

Purpose:

- standalone server hoặc một node trong mesh

Columns:

- `id text primary key`
- `user_id text not null`
- `name text not null`
- `public_ip inet null`
- `private_ip inet null`
- `agent_id text null`
- `status text not null default 'pending_enrollment'`
- `labels_json jsonb not null default '{}'`
- `runtime_capabilities_json jsonb not null default '{}'`
- `created_at timestamptz not null`
- `updated_at timestamptz not null`

Indexes and rules:

- unique index trên `(user_id, name)`
- index trên `agent_id`
- `public_ip` hoặc `private_ip` phải có ít nhất một giá trị
- `agent_id` nullable cho tới khi enroll thành công

#### `MeshNetwork`

Purpose:

- logical group of instances cho `distributed-mesh`

Columns:

- `id text primary key`
- `user_id text not null`
- `name text not null`
- `provider text not null`
- `cidr cidr not null`
- `status text not null default 'provisioning'`
- `created_at timestamptz not null`
- `updated_at timestamptz not null`

Indexes and rules:

- unique index trên `(user_id, name)`
- `provider` chỉ nhận `wireguard` hoặc `tailscale`

#### `Cluster`

Purpose:

- target cluster cho `distributed-k3s`

Columns:

- `id text primary key`
- `user_id text not null`
- `name text not null`
- `provider text not null`
- `kubeconfig_secret_ref text not null`
- `status text not null default 'validating'`
- `created_at timestamptz not null`
- `updated_at timestamptz not null`

Indexes and rules:

- unique index trên `(user_id, name)`
- `provider` chỉ nhận `k3s`
- lưu `kubeconfig_secret_ref`, không lưu raw kubeconfig trong row

### Projects and deploy contract

#### `Project`

Purpose:

- root entity cho repo, binding, revisions, topology và observability

Columns:

- `id text primary key`
- `user_id text not null`
- `name text not null`
- `slug text not null`
- `default_branch text not null`
- `created_at timestamptz not null`
- `updated_at timestamptz not null`

Indexes and rules:

- unique index trên `(user_id, slug)`
- index trên `user_id`

#### `ProjectRepoLink`

Purpose:

- bind một GitHub repo vào một project

Columns:

- `id text primary key`
- `project_id text not null`
- `github_installation_id text not null`
- `github_repo_id bigint not null`
- `repo_owner text not null`
- `repo_name text not null`
- `tracked_branch text not null`
- `preview_enabled boolean not null default false`
- `created_at timestamptz not null`
- `updated_at timestamptz not null`

Indexes and rules:

- unique index trên `project_id`
- index trên `(github_installation_id, github_repo_id)`
- repo link ownership được validate ở app layer bằng project owner + installation scope

#### `DeploymentBinding`

Purpose:

- logical mapping từ project sang target thật

Columns:

- `id text primary key`
- `project_id text not null`
- `name text not null`
- `target_ref text not null`
- `runtime_mode text not null`
- `target_kind text not null`
- `target_id text not null`
- `placement_policy_json jsonb not null default '{}'`
- `domain_policy_json jsonb not null default '{}'`
- `compatibility_policy_json jsonb not null default '{}'`
- `scale_to_zero_policy_json jsonb not null default '{}'`
- `created_at timestamptz not null`
- `updated_at timestamptz not null`

Indexes and rules:

- unique index trên `(project_id, target_ref)`
- index trên `(project_id, target_kind, target_id)`
- `runtime_mode` chỉ nhận `standalone`, `distributed-mesh`, `distributed-k3s`
- `target_kind` chỉ nhận `instance`, `mesh`, `cluster`
- không dùng DB FK polymorphic cho `target_id`; validation nằm ở app layer

#### `Service`

Purpose:

- metadata của service-level workload trong project

Columns:

- `id text primary key`
- `project_id text not null`
- `name text not null`
- `path text not null`
- `public boolean not null default false`
- `runtime_profile text null`
- `healthcheck_json jsonb not null default '{}'`
- `created_at timestamptz not null`
- `updated_at timestamptz not null`

Indexes and rules:

- unique index trên `(project_id, name)`
- index trên `project_id`

#### `Blueprint`

Purpose:

- compiled project contract trước khi sinh desired revision

Columns:

- `id text primary key`
- `project_id text not null`
- `source_kind text not null`
- `source_ref text not null`
- `compiled_json jsonb not null`
- `created_at timestamptz not null`

Indexes and rules:

- index trên `project_id`
- index trên `(project_id, source_kind, created_at desc)`
- immutable record, không có `updated_at`

### Revisions and rollout

#### `DesiredStateRevision`

Purpose:

- immutable desired runtime document đã được bind vào target

Columns:

- `id text primary key`
- `project_id text not null`
- `blueprint_id text not null`
- `deployment_binding_id text not null`
- `commit_sha text not null`
- `trigger_kind text not null`
- `status text not null default 'draft'`
- `compiled_revision_json jsonb not null`
- `created_at timestamptz not null`
- `updated_at timestamptz not null`

Indexes and rules:

- index trên `(project_id, created_at desc)`
- index trên `(deployment_binding_id, status)`
- index trên `(project_id, commit_sha)`

#### `Deployment`

Purpose:

- rollout execution record cho một revision

Columns:

- `id text primary key`
- `project_id text not null`
- `revision_id text not null`
- `status text not null default 'queued'`
- `started_at timestamptz null`
- `completed_at timestamptz null`
- `created_at timestamptz not null`
- `updated_at timestamptz not null`

Indexes and rules:

- index trên `(project_id, created_at desc)`
- index trên `(revision_id, status)`

#### `PreviewEnvironment`

Purpose:

- preview lifecycle cho pull request

Columns:

- `id text primary key`
- `project_id text not null`
- `revision_id text not null`
- `pr_number bigint not null`
- `status text not null default 'provisioning'`
- `domain_json jsonb not null default '{}'`
- `destroyed_at timestamptz null`
- `created_at timestamptz not null`
- `updated_at timestamptz not null`

Indexes and rules:

- unique index trên `(project_id, pr_number)`
- index trên `(revision_id, status)`

### Observability and FinOps

#### `TraceSummary`

Purpose:

- summarized distributed trace keyed by `correlation_id`

Columns:

- `id text primary key`
- `project_id text not null`
- `correlation_id text not null`
- `trace_json jsonb not null`
- `started_at timestamptz not null`
- `ended_at timestamptz not null`
- `created_at timestamptz not null`

Indexes and rules:

- unique index trên `(project_id, correlation_id)`
- index trên `(project_id, started_at desc)`

#### `NodeLatencyReport`

Purpose:

- aggregated latency per node-to-node or service edge window

Columns:

- `id text primary key`
- `project_id text not null`
- `source_node_id text not null`
- `target_node_id text not null`
- `service_edge text not null`
- `p95_ms integer not null`
- `max_ms integer not null`
- `window_started_at timestamptz not null`
- `window_ended_at timestamptz not null`
- `created_at timestamptz not null`

Indexes and rules:

- index trên `(project_id, window_started_at desc)`
- index trên `(source_node_id, target_node_id, window_started_at desc)`

#### `MetricRollup`

Purpose:

- storage-efficient aggregate window cho FinOps và long-term charts

Columns:

- `id text primary key`
- `project_id text not null`
- `target_kind text not null`
- `target_id text not null`
- `service_name text not null`
- `window text not null`
- `cpu_p95 double precision not null`
- `cpu_max double precision not null`
- `cpu_min double precision not null`
- `cpu_avg double precision not null`
- `ram_p95 double precision not null`
- `ram_max double precision not null`
- `ram_min double precision not null`
- `ram_avg double precision not null`
- `sample_count integer not null`
- `window_started_at timestamptz not null`
- `window_ended_at timestamptz not null`
- `created_at timestamptz not null`

Indexes and rules:

- unique index trên `(project_id, target_kind, target_id, service_name, window, window_started_at)`
- không lưu raw samples dài hạn

Decision note:

- Thêm `min` và `avg` vào schema triển khai để khớp cost model và project rules, dù backend guide chỉ nhấn mạnh `p95`, `max`, `min`, `avg`, `count` ở mức ingest semantics.

#### `RuntimeIncident`

Purpose:

- operational incident record cho unhealthy revision, rollout failure hoặc runtime degradation

Columns:

- `id text primary key`
- `project_id text not null`
- `revision_id text null`
- `severity text not null`
- `kind text not null`
- `summary text not null`
- `details_json jsonb not null default '{}'`
- `created_at timestamptz not null`

Indexes and rules:

- index trên `(project_id, created_at desc)`
- index trên `(revision_id, created_at desc)`
- `severity` chỉ nhận `info`, `warning`, `critical`

#### `TopologyNode`

Purpose:

- read model cho topology graph

Columns:

- `id text primary key`
- `project_id text not null`
- `node_type text not null`
- `node_ref text not null`
- `status text not null default 'unknown'`
- `metadata_json jsonb not null default '{}'`
- `created_at timestamptz not null`
- `updated_at timestamptz not null`

Indexes and rules:

- unique index trên `(project_id, node_ref)`
- index trên `(project_id, node_type)`

#### `TopologyEdge`

Purpose:

- read model cho service dependency hoặc network edge trong topology graph

Columns:

- `id text primary key`
- `project_id text not null`
- `source_node_ref text not null`
- `target_node_ref text not null`
- `edge_type text not null`
- `status text not null default 'unknown'`
- `latency_summary_json jsonb not null default '{}'`
- `created_at timestamptz not null`
- `updated_at timestamptz not null`

Indexes and rules:

- unique index trên `(project_id, source_node_ref, target_node_ref, edge_type)`
- index trên `(project_id, source_node_ref)`
- index trên `(project_id, target_node_ref)`

## Secret and Token Handling Rules

### Hash-only storage

Các giá trị sau phải lưu `hash`, không lưu raw:

- `password`
- `PersonalAccessToken`
- bootstrap tokens
- agent tokens
- session continuation tokens dạng bearer

Quy ước:

- Dùng one-way hash với salt/pepper phù hợp.
- Có thể lưu thêm `token_prefix` hoặc `fingerprint` ngắn cho UX/audit.
- Log chỉ được in prefix hoặc masked fingerprint, không in token đầy đủ.

### Secret-ref or encrypted-at-rest storage

Các giá trị sau không nên nằm trực tiếp trong row nghiệp vụ:

- `kubeconfig`
- GitHub App private key
- OAuth client secrets
- managed credentials cho runtime dependencies
- mesh private keys

Quy ước:

- Ưu tiên `secret_ref` tới secret manager hoặc encrypted config store.
- Chỉ fallback sang encrypted-at-rest blob khi chưa có external secret manager.
- Không trả secret raw trong API responses, operator stream, trace, incident hay logs.

### Logging and telemetry redaction

- `RequestLogger`, incident logs, webhook logs, agent control logs, build callback logs phải redact token và secret-bearing fields.
- `details_json`, `trace_json`, `metadata_json`, `latency_summary_json` không được nhét secret raw.
- Query params, headers như `Authorization`, PAT, installation token, bootstrap token phải bị redact trước khi vào structured logs.

## Domain Relationship Notes

Quan hệ chính:

- `User` 1:N `OAuthIdentity`
- `User` 1:N `PersonalAccessToken`
- `User` 1:N `GitHubInstallation`
- `User` 1:N `Project`
- `User` 1:N `Instance`
- `User` 1:N `MeshNetwork`
- `User` 1:N `Cluster`
- `Project` 0:1 `ProjectRepoLink`
- `Project` 1:N `DeploymentBinding`
- `Project` 1:N `Service`
- `Project` 1:N `Blueprint`
- `Project` 1:N `DesiredStateRevision`
- `Project` 1:N `Deployment`
- `Project` 1:N `PreviewEnvironment`
- `Project` 1:N `TraceSummary`
- `Project` 1:N `NodeLatencyReport`
- `Project` 1:N `MetricRollup`
- `Project` 1:N `RuntimeIncident`
- `Project` 1:N `TopologyNode`
- `Project` 1:N `TopologyEdge`
- `DesiredStateRevision` N:1 `Blueprint`
- `DesiredStateRevision` N:1 `DeploymentBinding`
- `Deployment` N:1 `DesiredStateRevision`
- `PreviewEnvironment` N:1 `DesiredStateRevision`

Quan hệ được resolve ở app layer thay vì hard DB FK:

- `DeploymentBinding.target_kind + target_id -> Instance | MeshNetwork | Cluster`
- `Instance.agent_id -> Agent`
- `TopologyEdge.source_node_ref/target_node_ref -> TopologyNode.node_ref`

Lý do:

- Tránh polymorphic FK phức tạp cho `target_kind/target_id`.
- Cho phép topology và telemetry ingest linh hoạt hơn, kể cả khi node/edge đến trước snapshot đầy đủ.
- Giữ backend control plane làm nơi resolve ownership và runtime authority thay vì để DB schema ép những quan hệ động.

## Stable Contracts Published for Other Teams

Day 2 coi các quyết định sau là đủ ổn định để các team khác bám theo:

- Tất cả entity IDs là string prefixed IDs.
- `project_id`, `binding_id`, `revision_id`, `deployment_id`, `trace_id`, `instance_id` trong API/event đều là external stable IDs, không phải numeric DB IDs.
- Policy-shaped fields dùng `jsonb` ở persistence và object JSON ở contract.
- `DeploymentBinding` là nguồn resolve `target_ref`.
- `ProjectRepoLink` là one-repo-per-project contract.
- Metrics lưu aggregate windows, không lưu raw long-term samples.
- `distributed-k3s` chỉ dùng `Cluster` + K3s-facing runtime driver; persistence không tạo shortcut nào cho direct workload control ngoài Kubernetes.

## Immediate Inputs for Day 3

- Refactor response envelope hiện tại từ `request_id` only sang chuẩn có thể mở rộng sang `correlation_id`.
- Chuyển `User` và `Agent` sang string IDs trước khi thêm domain tables mới.
- Dừng mở rộng `AutoMigrate`; bắt đầu versioned migrations.
- Tạo package home mới cho domain services thay vì tiếp tục dồn business logic vào `internal/service`.
