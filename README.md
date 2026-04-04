# LazyOps

LazyOps is a multi-surface deployment control plane with four main actors:

- backend API and persistence
- outbound runtime agents
- local operator CLI
- frontend operator experience

The repo is no longer a generic Gin boilerplate. It already carries product-specific contracts for:

- auth and CLI PAT flows
- GitHub App sync and repo linking
- target onboarding for `instance`, `mesh_network`, and `cluster`
- init validation and `DeploymentBinding`
- blueprint, revision, and deployment records
- build callbacks
- agent enrollment, heartbeat, and outbound control WebSocket
- topology and trace reads
- debug tunnel sessions

## Start Here

- Current-state docs hub: [`docs/index.md`](docs/index.md)
- Feature overview: [`docs/feature-catalog.md`](docs/feature-catalog.md)
- Canonical contract matrix: [`docs/contracts-matrix.md`](docs/contracts-matrix.md)
- Manual test playbook: [`docs.md`](docs.md)

## Canonical Surface Notes

- Public control sockets are `GET /ws/agents/control` and `GET /ws/operators/stream`.
- The `/api/v1` WebSocket variants remain compatibility aliases only.
- The old legacy user-agent demo stream is not the canonical agent/backend control surface.
- Command envelopes must use exact command constants such as `start_release_candidate`, not dotted aliases.

## Repo Layout

```text
agent/      outbound runtime bridge
backend/    control-plane API and persistence
cli/        operator CLI
guide/      narrative product and implementation guides
docs/       current-state summary docs
docs.md     manual test playbook
```

## Quick Start

### Backend

```bash
cd backend
go run ./cmd/server
```

### CLI

```bash
cd cli
go run ./cmd/lazyops login
```

### Agent

```bash
cd agent
go run ./cmd/server
```

## Verification

```bash
cd backend && go test ./...
cd cli && go test ./...
cd agent && go test ./...
```
