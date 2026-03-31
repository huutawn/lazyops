# CLI Day 1 Rule Lock and Command Surface

## Summary

Day 1 locks two things for the CLI track:

- the v1 public command surface
- the non-negotiable compliance checklist derived from the source guides

This document is an internal execution reference for the CLI team. It does not replace the source guides. If this file conflicts with the source guides, the guides win.

Day 1 conclusion:

- The CLI remains a local operator tool, not the main deployment trigger.
- The v1 command surface is now fixed so the team can build without drifting into ad-hoc commands.
- `lazyops init` is confirmed as the center of CLI onboarding and must produce logical deploy intent only.
- The CLI must support all three runtime modes: `standalone`, `distributed-mesh`, and `distributed-k3s`.
- Tunnel commands remain optional debug tools and must not become a production access path.
- The CLI can proceed in parallel with backend and frontend as long as it stays within the locked contracts and uses mocks when needed.

## Source Documents

- `guide/cli-guide.md`
- `guide/lazyops-implementation-master-plan.md`
- `guide/lazyops-master-des.md`
- `cli-task.md`

## Current Baseline

- `cli/` is still greenfield and currently contains only an empty `go.mod`.
- No CLI command tree, config layer, transport layer, or internal docs existed before this Day 1 output.
- The CLI track therefore starts by locking scope and rules before any implementation begins.

## Locked v1 Command Surface

The following commands are in scope for CLI v1 and must not be removed or renamed without updating the execution plan and reconciling with the source guides.

### Project setup commands

- `lazyops login`
- `lazyops logout`
- `lazyops init`
- `lazyops link`
- `lazyops doctor`

### Observe and operate commands

- `lazyops status`
- `lazyops bindings`
- `lazyops logs <service>`
- `lazyops traces <correlation-id>`

### Optional debug commands

- `lazyops tunnel db`
- `lazyops tunnel tcp`

## Command Intent Lock

| Command | Intent | Required boundary |
| --- | --- | --- |
| `lazyops login` | Authenticate once and store CLI identity | Must use PAT from backend and store it in keychain or protected local file |
| `lazyops logout` | Revoke or clear local CLI identity | Must remove local credentials and may call PAT revoke |
| `lazyops init` | Turn the repo into a valid LazyOps deploy contract | Must scan repo, detect services, fetch targets, choose runtime mode, create or attach binding, review config, write `lazyops.yaml` |
| `lazyops link` | Connect local repo to project, GitHub App repo, and binding | Must verify repo ownership, project ownership, GitHub App installation, and target availability |
| `lazyops doctor` | Validate local onboarding and deploy contract health | Must check auth, repo link, `lazyops.yaml`, binding existence, dependency declarations, and webhook health |
| `lazyops status` | Show a thin runtime summary | Must aggregate backend state, not act as a deploy engine |
| `lazyops bindings` | List and reuse deployment bindings | Must operate on logical binding state, not direct infra coordinates |
| `lazyops logs <service>` | Inspect runtime logs for a service | Must use the logs stream contract and avoid leaking secrets |
| `lazyops traces <correlation-id>` | Inspect distributed request flow | Must use the trace summary contract and show correlation path clearly |
| `lazyops tunnel db` | Open a debug database tunnel | Must remain an operator debug tool and not become a default access path |
| `lazyops tunnel tcp` | Open a generic debug TCP tunnel | Must remain an operator debug tool and not bypass the runtime model |

## Commands Explicitly Out of Scope for v1

The following command shapes are not part of the locked v1 CLI surface:

- `lazyops deploy`
- `lazyops build`
- `lazyops rollback`
- `lazyops runtime`
- `lazyops ssh`
- any command that directly performs infrastructure deployment from the local machine
- any command that requires users to author raw infrastructure YAML
- any command that forces users to classify services as `frontend` or `backend`

These are excluded because the product contract says deploy is mainly driven by GitHub `push` and `pull_request`, while the CLI acts as the onboarding and operator surface.

## Non-Negotiable Compliance Checklist

Every CLI task from Day 2 onward must continue to satisfy all items below.

### Product role and delivery model

- The CLI is a local operator tool.
- The CLI is not the main deploy trigger.
- The main deploy triggers are GitHub `push` and `pull_request` after onboarding.
- The CLI must stay thin and Git-like: log in once, use many times.
- The CLI must be buildable in parallel with backend, agent, and frontend.
- The CLI must be able to onboard targets without waiting for frontend completion.

### Runtime and workload model

- The CLI must support `standalone`, `distributed-mesh`, and `distributed-k3s`.
- The CLI must treat every workload as a `service`.
- The CLI must never force a `frontend` versus `backend` classification.
- The CLI must not introduce a K3s-first mental model.
- The CLI must respect the K3s boundary: `distributed-k3s` uses Kubernetes as the workload orchestrator.

### `lazyops init` and deploy contract rules

- `lazyops init` is the key onboarding command.
- `init` must scan the repo.
- `init` must detect services from common repo signals like `package.json`, `go.mod`, `requirements.txt`, `Dockerfile`, and monorepo layout.
- `init` must fetch targets from backend-provided `instances`, `mesh networks`, and `clusters`.
- `init` must let the user choose `standalone`, `distributed-mesh`, or `distributed-k3s`.
- `init` must create or attach a `DeploymentBinding`.
- `init` must review config before writing.
- `init` must write `lazyops.yaml`.
- `lazyops.yaml` is the deploy contract.
- `lazyops.yaml` must contain logical intent only.
- `lazyops.yaml` must not store SSH credentials, raw kubeconfig, server passwords, private keys, PATs, or long-lived GitHub credentials.
- `lazyops.yaml` must not use hard-coded server IPs as the source of truth.
- Repo-to-target mapping must be resolved by backend through `DeploymentBinding`.

### Security and secret handling

- PATs, tokens, and secrets must never be logged in plaintext.
- CLI credential storage must prefer the OS keychain.
- If keychain is unavailable, the fallback credentials file must be protected by strict file permissions.
- The CLI must not store SSH credentials for normal operations.
- The CLI must not write raw infrastructure secrets into repo files.

### UX expectations

- Any request taking longer than one second must show spinner feedback.
- Config review must be clear before writing `lazyops.yaml`.
- Destructive actions must require confirmation.
- Error messages must clearly tell the user what to fix next.

### Observability and debug boundaries

- The CLI must include logs, traces, status, and bindings commands in v1.
- Tunnel commands are optional debug tools only.
- Tunnel commands must not become the main access path for production usage.

## Early Contract Dependencies

The CLI track can move immediately after Day 1, but the following contracts must be considered locked or requested early:

- `POST /api/v1/auth/cli-login`
- `POST /api/v1/auth/pat/revoke`
- `GET /api/v1/projects`
- `GET /api/v1/instances`
- `GET /api/v1/mesh-networks`
- `GET /api/v1/clusters`
- `POST /api/v1/projects/:id/deployment-bindings`
- `GET /api/v1/projects/:id/deployment-bindings`
- repo-link contract for `lazyops link`
- `lazyops.yaml` schema or validation route/package
- `GET /api/v1/traces/:correlation_id`
- `GET /ws/logs/stream`
- tunnel session contract for `tunnel db`
- tunnel session contract for `tunnel tcp` or a generic tunnel endpoint
- dedicated `status` contract or a stable composition rule over topology and deployment APIs

## Day 1 Exit Criteria

Day 1 is considered complete only if all of the following are true:

- the v1 command surface is explicitly listed and frozen
- command intent boundaries are documented
- out-of-scope command types are documented to prevent CLI drift
- the non-negotiable compliance checklist is extracted into an internal CLI document
- `cli-task.md` Day 1 tasks are updated from `(pending)` to `(done)`

## Next Step After Day 1

The next implementation step is Day 2:

- bootstrap the Go CLI module and command tree
- add output and spinner abstractions
- create a mock transport path so the CLI can proceed before backend completion
