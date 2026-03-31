# Backend Day 3 API Contract Foundation

## Summary

Day 3 khóa contract API nền cho backend theo 5 mục trong `backend-task.md`:

- response envelope chuẩn cho success và error
- auth middleware contract cho web session, bearer token và internal agent-scoped auth
- permission guard matrix cho `viewer`, `operator`, `admin`, project ownership và target ownership
- auth request/response schema, error codes và permission semantics cho frontend và CLI
- merge checklist để mọi API mới dùng cùng contract

Tài liệu này kế thừa:

- `backend-day1-audit.md`
- `backend-day2-persistence-foundation.md`
- `guide/backend-guide.md`
- `guide/lazyops-implementation-master-plan.md`
- `guide/lazyops-master-des.md`
- `guide/project-rules.md`

## Current Baseline vs Locked Contract

Baseline hiện tại trong code:

- response envelope chỉ có `success`, `message`, `request_id`, `data`, `error`
- `request_id` hiện lấy từ `X-Request-ID`
- auth middleware hiện chỉ có `Bearer` JWT cho user routes
- role guard hiện chỉ check role, chưa có ownership guard
- IDs trong current code vẫn là `uint`, nhưng Day 2 đã khóa target contract là string IDs dạng `prefix + ULID`

Day 3 quyết định:

- contract công khai cho API từ nay phải đi theo Day 2, kể cả khi implementation hiện thời còn đang ở baseline cũ
- mọi contract mới phải ưu tiên `request_id` và `correlation_id`
- auth context phải tách rõ `user session`, `CLI PAT`, `agent token`
- permission không chỉ là role; phải có thêm ownership scope

## Response Envelope Contract

### Success envelope

Chuẩn success chính thức:

```json
{
  "success": true,
  "message": "login successful",
  "request_id": "req_01H...",
  "correlation_id": "corr_01H...",
  "data": {},
  "meta": {
    "version": "v1"
  }
}
```

Quy ước:

- `success`: luôn là `true`
- `message`: human-readable summary ngắn
- `request_id`: per-request ID, luôn có với HTTP response
- `correlation_id`: end-to-end flow ID; với public HTTP requests phải luôn có khi middleware correlation được đưa vào
- `data`: payload chính
- `meta`: optional, dùng cho pagination, version, warnings, deprecation hints

### Error envelope

Chuẩn error chính thức:

```json
{
  "success": false,
  "message": "login failed",
  "request_id": "req_01H...",
  "correlation_id": "corr_01H...",
  "error": {
    "code": "invalid_credentials",
    "details": "email or password is incorrect",
    "fields": {
      "email": "invalid"
    },
    "retryable": false
  }
}
```

Quy ước:

- `success`: luôn là `false`
- `message`: user-facing summary
- `error.code`: machine-readable stable key
- `error.details`: string hoặc object nhỏ, không chứa secret
- `error.fields`: optional field-level validation errors
- `error.retryable`: optional boolean cho client retry policy

### HTTP status mapping

| Category | HTTP status | Example `error.code` |
| --- | --- | --- |
| validation error | `400` | `invalid_request`, `invalid_input`, `invalid_payload` |
| unauthenticated | `401` | `missing_bearer_token`, `invalid_token`, `expired_token`, `missing_session` |
| forbidden | `403` | `insufficient_permissions`, `project_access_denied`, `target_access_denied` |
| not found | `404` | `project_not_found`, `binding_not_found`, `repo_link_not_found` |
| conflict | `409` | `email_already_exists`, `duplicate_target_ref`, `repo_already_linked` |
| unprocessable business rule | `422` | `runtime_mode_mismatch`, `invalid_dependency_binding` |
| rate limit | `429` | `rate_limited` |
| system error | `500` | `internal_error`, `database_error`, `provider_error` |
| upstream unavailable | `502/503` | `github_unavailable`, `cluster_unreachable`, `runtime_unavailable` |

### Meta contract

`meta` chỉ dùng cho các trường hợp sau:

- pagination: `page`, `page_size`, `total`
- API version hint: `version`
- deprecation warning: `deprecated`, `replacement`
- partial results: `partial`, `warnings`

Không dùng `meta` để đặt business payload thay cho `data`.

## Auth Middleware Contract

## 1. Auth kinds

Ba loại auth phải được support ở contract level:

- `web_session`
- `cli_pat`
- `agent_token`

### `web_session`

Purpose:

- auth cho web UI

Transport:

- ưu tiên `Authorization: Bearer <jwt>` trong baseline hiện tại
- có thể mở rộng sang secure session cookie sau mà không đổi auth context schema

Context shape:

```json
{
  "auth_kind": "web_session",
  "subject_type": "user",
  "subject_id": "usr_01H...",
  "email": "user@example.com",
  "role": "operator"
}
```

Applies to:

- routes user-facing cho web app

### `cli_pat`

Purpose:

- auth cho CLI commands

Transport:

- `Authorization: Bearer <pat>`

Context shape:

```json
{
  "auth_kind": "cli_pat",
  "subject_type": "user",
  "subject_id": "usr_01H...",
  "role": "operator",
  "token_id": "pat_01H..."
}
```

Applies to:

- CLI flows như `lazyops init`, target listing, PAT revoke, project access

### `agent_token`

Purpose:

- auth cho agent enrollment follow-up, control channel, telemetry ingest

Transport:

- `Authorization: Bearer <agent_token>`

Context shape:

```json
{
  "auth_kind": "agent_token",
  "subject_type": "agent",
  "subject_id": "agt_01H...",
  "instance_id": "inst_01H...",
  "capabilities": {}
}
```

Applies to:

- `GET /ws/agents/control`
- telemetry ingest
- runtime command acknowledgement

### Middleware responsibilities

Middleware auth chuẩn phải làm được:

- parse auth kind từ token/source phù hợp
- attach normalized auth context vào request context
- không log raw token
- distinguish `missing credentials` với `invalid credentials`
- trả về envelope error chuẩn theo Day 3

### Standard auth context

Key nội bộ đề xuất:

- `auth.context`

Minimal fields:

- `auth_kind`
- `subject_type`
- `subject_id`
- `role`
- `email` nếu là user
- `token_id` nếu là PAT
- `agent_id` hoặc `instance_id` nếu là agent

### Auth failure codes

| Case | HTTP | `error.code` |
| --- | --- | --- |
| missing bearer | `401` | `missing_bearer_token` |
| malformed bearer | `401` | `invalid_authorization_header` |
| bad JWT/PAT/agent token | `401` | `invalid_token` |
| expired session or PAT | `401` | `expired_token` |
| revoked PAT or identity | `401` | `revoked_token` |
| wrong auth kind for endpoint | `403` | `auth_kind_not_allowed` |

## Permission Guard Matrix

### Role semantics

| Role | Meaning |
| --- | --- |
| `viewer` | read-only access to owned resources |
| `operator` | read + operational actions on owned resources |
| `admin` | full management on owned resources and administrative actions within owned tenant scope |

Role không thay thế ownership.

### Ownership scopes

Hai ownership scopes bắt buộc ở API layer:

- `project ownership`
- `target ownership`

Rules:

- user chỉ được thao tác trên `Project`, `Instance`, `MeshNetwork`, `Cluster`, `DeploymentBinding`, `ProjectRepoLink` do chính user sở hữu
- repo link phải pass cả `project ownership` và `github installation access`
- binding creation phải pass cả `project ownership` và `target ownership`
- trace/topology/incident query phải pass `project ownership`

### Guard matrix

| Resource / action | Viewer | Operator | Admin | Ownership required |
| --- | --- | --- | --- | --- |
| `GET /users/me` | allow | allow | allow | authenticated user |
| `POST /api/v1/auth/pat/revoke` | allow | allow | allow | token belongs to current user |
| `GET /api/v1/projects` | allow | allow | allow | only own projects returned |
| `POST /api/v1/projects` | deny | allow | allow | current user owner of created project |
| `POST /api/v1/projects/:id/repo-link` | deny | allow | allow | project ownership + installation access |
| `GET /api/v1/instances` | allow | allow | allow | only own targets returned |
| `POST /api/v1/instances` | deny | allow | allow | current user owner of target |
| `POST /api/v1/mesh-networks` | deny | allow | allow | target ownership |
| `POST /api/v1/clusters` | deny | allow | allow | target ownership |
| `POST /api/v1/projects/:id/deployment-bindings` | deny | allow | allow | project ownership + target ownership |
| `POST /api/v1/projects/:id/deployments` | deny | allow | allow | project ownership |
| `GET /api/v1/projects/:id/topology` | allow | allow | allow | project ownership |
| `GET /api/v1/traces/:correlation_id` | allow | allow | allow | project ownership |
| agent telemetry ingest | deny | deny | deny | `agent_token` only |
| operator stream | deny | allow | allow | authenticated owned scope |

### Guard categories to implement

Backend từ Day 4 trở đi phải có các guard loại sau:

- `RequireAuthenticatedUser`
- `RequireAuthKind(auth_kinds...)`
- `RequireRoles(roles...)`
- `RequireProjectOwnership`
- `RequireTargetOwnership`
- `RequireTokenOwnership`
- `RequireGitHubInstallationAccess`

## Auth Request/Response Schemas

### 1. `POST /api/v1/auth/register`

Request:

```json
{
  "name": "Jane Doe",
  "email": "jane@example.com",
  "password": "StrongPass123!"
}
```

Response `201`:

```json
{
  "success": true,
  "message": "register successful",
  "request_id": "req_01H...",
  "correlation_id": "corr_01H...",
  "data": {
    "access_token": "<jwt>",
    "token_type": "Bearer",
    "expires_in": 86400,
    "user": {
      "id": "usr_01H...",
      "display_name": "Jane Doe",
      "email": "jane@example.com",
      "role": "viewer",
      "status": "active"
    }
  }
}
```

Error codes:

- `invalid_input`
- `email_already_exists`
- `internal_error`

### 2. `POST /api/v1/auth/login`

Request:

```json
{
  "email": "jane@example.com",
  "password": "StrongPass123!"
}
```

Response `200`:

- cùng shape với `register`

Error codes:

- `invalid_input`
- `invalid_credentials`
- `account_disabled`
- `internal_error`

### 3. `POST /api/v1/auth/cli-login`

Request:

```json
{
  "auth_flow": "password",
  "email": "jane@example.com",
  "password": "StrongPass123!",
  "device_name": "macbook-pro"
}
```

Alternative request:

- sau OAuth success, backend có thể issue PAT từ authenticated session, nhưng response shape phải giữ nguyên

Response `200`:

```json
{
  "success": true,
  "message": "cli login successful",
  "request_id": "req_01H...",
  "correlation_id": "corr_01H...",
  "data": {
    "token": "<pat>",
    "token_type": "Bearer",
    "token_id": "pat_01H...",
    "expires_at": "2026-04-30T12:00:00Z",
    "user": {
      "id": "usr_01H...",
      "display_name": "Jane Doe",
      "email": "jane@example.com",
      "role": "operator",
      "status": "active"
    }
  }
}
```

Error codes:

- `invalid_input`
- `invalid_credentials`
- `account_disabled`
- `rate_limited`

CLI semantics:

- CLI phải treat PAT như secret write-once
- CLI chỉ cần `token`, `token_type`, `expires_at`, `user`

### 4. `POST /api/v1/auth/pat/revoke`

Auth:

- `web_session` hoặc `cli_pat`

Request:

```json
{
  "token_id": "pat_01H..."
}
```

Response `200`:

```json
{
  "success": true,
  "message": "pat revoked",
  "request_id": "req_01H...",
  "correlation_id": "corr_01H...",
  "data": {
    "token_id": "pat_01H...",
    "revoked": true
  }
}
```

Error codes:

- `invalid_input`
- `token_not_found`
- `token_access_denied`

### 5. `GET /api/v1/auth/oauth/google/start`

Auth:

- public

Response semantics:

- redirect to provider
- nếu frontend cần JSON mode sau này, contract phụ trợ có thể là:

```json
{
  "success": true,
  "message": "redirect ready",
  "request_id": "req_01H...",
  "correlation_id": "corr_01H...",
  "data": {
    "provider": "google",
    "authorization_url": "https://accounts.google.com/..."
  }
}
```

Error codes:

- `oauth_not_configured`
- `internal_error`

### 6. `GET /api/v1/auth/oauth/google/callback`

Success semantics:

- backend validates `state`
- backend find-or-create user
- backend creates `web_session`
- backend redirects to frontend success path hoặc trả JSON success theo integration mode

Failure codes:

- `invalid_oauth_state`
- `oauth_provider_error`
- `account_disabled`

### 7. `GET /api/v1/auth/oauth/github/start`

Semantics:

- giống Google start nhưng `provider = github`

Failure codes:

- `oauth_not_configured`
- `internal_error`

### 8. `GET /api/v1/auth/oauth/github/callback`

Semantics:

- giống Google callback
- GitHub OAuth2 dùng cho identity; repo permissions không được suy ra từ callback này

Failure codes:

- `invalid_oauth_state`
- `oauth_provider_error`
- `account_disabled`

### 9. `GET /api/v1/users/me`

Auth:

- `web_session` hoặc `cli_pat`

Response `200`:

```json
{
  "success": true,
  "message": "profile fetched",
  "request_id": "req_01H...",
  "correlation_id": "corr_01H...",
  "data": {
    "id": "usr_01H...",
    "display_name": "Jane Doe",
    "email": "jane@example.com",
    "role": "operator",
    "status": "active"
  }
}
```

## Standard Error Codes

Nhóm auth:

- `invalid_input`
- `invalid_request`
- `invalid_payload`
- `invalid_credentials`
- `missing_bearer_token`
- `invalid_authorization_header`
- `invalid_token`
- `expired_token`
- `revoked_token`
- `missing_session`
- `account_disabled`
- `email_already_exists`
- `token_not_found`
- `token_access_denied`
- `auth_kind_not_allowed`
- `insufficient_permissions`

Nhóm ownership:

- `project_access_denied`
- `target_access_denied`
- `repo_access_denied`
- `binding_access_denied`

Nhóm provider/upstream:

- `oauth_not_configured`
- `oauth_provider_error`
- `github_unavailable`
- `provider_error`

Nhóm system:

- `internal_error`
- `database_error`
- `rate_limited`

## Frontend and CLI Semantics

### Frontend

- Frontend chỉ dựa vào envelope chuẩn và `error.code`, không parse free-form `message` để điều khiển UX.
- OAuth start/callback phải coi là provider redirect flow, không assume luôn là JSON API flow.
- `role` và ownership là hai khái niệm khác nhau; UI có thể hiện action theo role nhưng backend mới là source of truth cho ownership.

### CLI

- CLI dùng `cli_pat` cho session continuity.
- CLI phải coi `expired_token` và `revoked_token` là tín hiệu yêu cầu login lại.
- CLI không được assume numeric IDs; mọi `id` trả về từ backend là string IDs.
- CLI nên log `request_id` khi gặp lỗi để hỗ trợ support/debug.

## API Merge Checklist

Mọi API mới từ Day 3 trở đi phải pass checklist này trước khi merge:

- Có success envelope chuẩn với `success`, `message`, `request_id`, `data`.
- Có error envelope chuẩn với `error.code` machine-readable.
- Không trả secret, token raw, private key, kubeconfig raw trong `data`, `error`, `meta`.
- Đã xác định auth kind hợp lệ cho endpoint: `public`, `web_session`, `cli_pat`, `agent_token`.
- Đã xác định role guard nếu endpoint không phải public.
- Đã xác định ownership guard nếu endpoint chạm vào project, target, repo link, binding hoặc token.
- Đã có mapping rõ cho `400`, `401`, `403`, `404`, `409`, `422`, `429`, `500/503` nếu phù hợp.
- Nếu endpoint dùng provider ngoài như GitHub hoặc OAuth thì đã chuẩn hóa `provider_error` path.
- Nếu endpoint trả list thì `meta` phải đủ chỗ cho pagination hoặc explicit note là unpaginated.
- Nếu endpoint tạo hoặc mutate resource thì response phải trả stable string IDs theo Day 2.
- Nếu endpoint public hoặc đi qua gateway thì phải tương thích với `correlation_id` propagation plan.
- Nếu endpoint dùng internal auth hoặc telemetry, phải redact token/secret khỏi logs.

## Immediate Inputs for Day 4

- Update current response helpers để hỗ trợ `correlation_id`, structured `error.code` và `meta`.
- Tách auth middleware thành contract-aware middleware thay vì chỉ có một `Authenticate` dùng JWT.
- Chuyển `Claims.UserID` và `UserProfile.ID` khỏi `uint` sang string ID theo Day 2 trước khi mở thêm auth surfaces.
- Bổ sung ownership guard primitives song song với cải thiện email/password auth.
