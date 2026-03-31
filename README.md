# lazyops-server

Boilerplate Golang/Gin theo huong doanh nghiep cho LazyOps, da ket noi PostgreSQL, auth JWT login bang email, RBAC, middleware co ban cho production, API versioning va WebSocket demo realtime.

## Kien truc

```text
cmd/server
config
internal/
  api/
    middleware/
    response/
    v1/
      controller/
  bootstrap/
  config/
  hub/
  models/
  repository/
  service/
pkg/
```

Luong xu ly chinh:

```text
route -> middleware -> controller -> service -> repository -> postgres
```

## Tinh nang

- PostgreSQL qua GORM + auto migrate
- JWT auth voi login bang email/password
- RBAC 3 role: `admin`, `operator`, `viewer`
- Seed tai khoan admin ban dau
- Middleware: request id, recovery, structured logging, CORS, security headers, rate limit, timeout
- API versioning qua prefix `/api/v1`
- WebSocket stream demo cho use case cap nhat trang thai agent realtime

## Bien moi truong quan trong

```bash
APP_NAME=lazyops-server
APP_ENV=development
SERVER_HOST=0.0.0.0
SERVER_PORT=8080
SERVER_REQUEST_TIMEOUT=15s
GIN_MODE=debug

DB_HOST=127.0.0.1
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=lazyops
DB_SSLMODE=disable
DB_TIMEZONE=Asia/Bangkok
DB_MAX_IDLE_CONNS=10
DB_MAX_OPEN_CONNS=50

JWT_SECRET=change-me-in-production
JWT_ISSUER=lazyops-server
JWT_EXPIRES_IN=24h

ALLOWED_ORIGINS=*
RATE_LIMIT_RPS=10
RATE_LIMIT_BURST=20

SEED_ADMIN_EMAIL=admin@lazyops.local
SEED_ADMIN_PASSWORD=ChangeMe123!
SEED_ADMIN_NAME=System Admin

WS_READ_BUFFER_SIZE=1024
WS_WRITE_BUFFER_SIZE=1024
WS_PING_PERIOD=30s
WS_PONG_WAIT=60s
```

## Chay project

```bash
go run ./cmd/server
```

## API chinh

- `GET /api/v1/health`
- `POST /api/v1/auth/register`
- `POST /api/v1/auth/login`
- `GET /api/v1/users/me`
- `GET /api/v1/agents`
- `POST /api/v1/agents` (`admin`, `operator`)
- `PUT /api/v1/agents/:agentID/status` (`admin`, `operator`)
- `GET /api/v1/ws/agents`

## Demo use case WebSocket

1. Dang nhap bang email/password de lay JWT.
2. Ket noi WebSocket toi `GET /api/v1/ws/agents` voi header `Authorization: Bearer <token>`.
3. Goi `PUT /api/v1/agents/agent-01/status` hoac gui event WebSocket:

```json
{
  "type": "agent.status.update",
  "agent_id": "agent-01",
  "name": "Agent 01",
  "status": "online"
}
```

4. Moi client dang subscribe se nhan event `agent.status.changed` theo realtime.

## Seed mac dinh

Khi app khoi dong, he thong se tao admin neu email seed chua ton tai trong database.
