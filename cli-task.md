# CLI 30-Day Task Tracker

## Rule Lock

Required source documents for every task in this file:

- `guide/cli-guide.md`
- `guide/lazyops-implementation-master-plan.md`
- `guide/lazyops-master-des.md`

If any task, implementation note, or status update in this file conflicts with the three documents above, the task must be corrected to match the guides. The guides must not be changed just to justify the task.

Current baseline:

- `cli/` is still greenfield and currently only contains an empty `go.mod`.
- This track is structured as a live 30-day tracker to take the CLI from scaffold level to a complete CLI v1 aligned with the spec.
- The main deployment triggers remain GitHub `push` and `pull_request`; the CLI is a local operator tool, not a deploy engine.

Hard rules that must not be violated by any task:

- The CLI must stay thin and feel like Git: log in once, use many times. It must not include a local build pipeline, a hidden deploy engine, or become the primary deploy trigger.
- The CLI exists to log in, initialize the deploy contract, link the repo, choose or create deployment bindings, validate health, inspect logs, traces, and rollout status, and open optional debug tunnels.
- `lazyops.yaml` is the project deploy contract, but it must contain logical intent only. It must never store SSH credentials, raw kubeconfig, server passwords, private keys, PATs, long-lived GitHub credentials, or hard-coded server IPs as the source of truth.
- Mapping from the repo to the real target must always go through a backend-managed `DeploymentBinding`; the repo stores only `target_ref`, and the backend is the place that resolves the actual target.
- Every workload must be modeled as a `service`; the CLI must never force the user to classify a workload as `frontend` or `backend`.
- The required runtime modes are `standalone`, `distributed-mesh`, and `distributed-k3s`.
- `init` is the central onboarding command and must always follow this flow: scan repo -> detect services -> fetch targets -> choose runtime mode -> create or attach `DeploymentBinding` -> review config -> write `lazyops.yaml`.
- `init` must not store SSH credentials, must not write infrastructure secrets, must not force the user to author raw infrastructure YAML, and must not place deploy authority into the repo.
- PATs must be stored in the OS keychain when possible; only if that is unavailable may the CLI fall back to a protected local credentials file with strict permissions.
- Plaintext `PAT`, tokens, managed secrets, tunnel credentials, or any other secrets must never appear in output, logs, debug traces, or error messages.
- Any request that takes longer than one second must show a spinner or equivalent feedback; config review must be clear before writing `lazyops.yaml`; destructive actions must require confirmation; error messages must state the next fix step clearly.
- Tunnel commands are optional debug features only; they must not become a production access pattern or bypass the runtime model.
- The CLI must be able to onboard targets without waiting for the frontend; if backend contracts are not ready yet, the CLI must use mock contracts or fixtures that match the locked schema so the track can continue.

Command surface that must be locked early:

- `lazyops login`
- `lazyops logout`
- `lazyops init`
- `lazyops link`
- `lazyops doctor`
- `lazyops status`
- `lazyops bindings`
- `lazyops logs <service>`
- `lazyops traces <correlation-id>`
- `lazyops tunnel db`
- `lazyops tunnel tcp`

Shared contracts that must be locked early:

- Auth PAT response from `POST /api/v1/auth/cli-login`
- Project list contracts
- Target summary models for `Instance`, `MeshNetwork`, and `Cluster`
- `DeploymentBinding` create/list schema
- `lazyops.yaml` schema or validation contract
- Repo-link contract
- Trace summary contract
- Logs stream contract
- Tunnel session contract
- Add `GET /api/v1/projects/:id/deployment-bindings`
- Add a dedicated status contract or a stable rule for composing status from topology/deployment APIs
- Add `POST /api/v1/tunnels/tcp/sessions` or a generic tunnel session contract

Local types that should exist early:

- `CredentialStore`
- `RepoScanResult`
- `ServiceCandidate`
- `InitPlan`
- `DoctorReport`
- `StatusSummary`

## Status Update Rules

- Every task must end with exactly one status: `(pending)`, `(doing)`, `(done)`, or `(blocked: <short reason>)`.
- All newly created tasks start as `(pending)`.
- When work starts on a task, only that task should be changed to `(doing)`.
- A task may be changed to `(done)` only when the related code or document work is complete, the relevant guides have been checked again, and the appropriate test, fixture, or verification step has been completed.
- If the task is blocked by a dependency, infrastructure, secrets, spec gap, or review blocker, change it to `(blocked: <short reason>)`.
- Do not keep more than one task in `(doing)` status within the same day section.
- If an earlier task is not finished or is `blocked`, dependent tasks after it must remain `(pending)`.
- Do not delete recorded tasks; only update their status in place to preserve execution history.
- Do not move tasks to another day just to make progress look cleaner. If work carries over, keep the task on its original day and add a follow-up task on the next day only if it is truly needed.
- When a task publishes a contract or schema, that contract must be stable enough for backend, agent, frontend, or other CLI submodules to use with mocks or parallel implementation.
- If a task touches auth, PAT storage, repo linking, binding resolution, `lazyops.yaml`, logs, traces, tunnels, or security redaction, it must be checked against `Rule Lock` again before it can be marked `(done)`.

Update examples:

- `- Finish CredentialStore with keychain and fallback file permissions. (doing)`
- `- Finish CredentialStore with keychain and fallback file permissions. (done)`
- `- Lock the deployment bindings list contract. (blocked: backend has not published the endpoint yet)`

## Day 1 - Rule Lock and Command Surface

Goal: lock the v1 command surface and the compliance checklist so the entire roadmap stays aligned with the guides.

- Lock the v1 command surface covering `login`, `logout`, `init`, `link`, `doctor`, `status`, `bindings`, `logs`, `traces`, `tunnel db`, and `tunnel tcp`. (done)
- Extract the non-negotiable rules checklist from `guide/cli-guide.md`, `guide/lazyops-implementation-master-plan.md`, and `guide/lazyops-master-des.md` into internal CLI documentation. (done)

## Day 2 - CLI Bootstrap and Mock Transport

Goal: build the Go CLI skeleton and a mock path so the team can move in parallel with the backend.

- Bootstrap the Go module for the CLI, the command tree, the output abstraction, and the spinner abstraction. (done)
- Build mock transport so the CLI can progress in parallel under `RULE 23` and `RULE 25` even when backend contracts are not ready yet. (done)

## Day 3 - Credential Storage and Secret Redaction

Goal: lock down secure PAT storage and ensure secrets never leak through any output path.

- Design `CredentialStore` to prefer the OS keychain and fall back to a protected local credentials file with strict permissions. (pending)
- Add redaction for PATs, tokens, and secrets in logs, debug output, and error output. (pending)

## Day 4 - Login Flow

Goal: complete `lazyops login` against the `POST /api/v1/auth/cli-login` contract.

- Implement `lazyops login` for email/password and browser OAuth entrypoints that receive a PAT from `POST /api/v1/auth/cli-login`. (pending)
- Add secret masking, spinner behavior for requests over one second, and actionable auth errors. (pending)

## Day 5 - Logout and Auth Guard

Goal: complete the auth lifecycle with revoke/session cleanup and a shared auth gate for commands that require login.

- Implement `lazyops logout` with PAT revoke when available or local session cleanup when only local credentials exist. (pending)
- Add a shared auth guard for every command that requires login before calling the backend. (pending)

## Day 6 - Client Models and Fixtures

Goal: lock models and fixtures so the CLI is not blocked by backend implementation timing.

- Lock client models for `Project`, `GitHubInstallation`, `Instance`, `MeshNetwork`, `Cluster`, and `DeploymentBinding`. (pending)
- Create fixtures or mock responses for project list, target list, binding create/list, trace query, and logs stream. (pending)

## Day 7 - Repo Scanner

Goal: scan repos using common signals and operate only with the `service` abstraction.

- Implement the repo scanner to detect repo root, monorepo layout, `package.json`, `go.mod`, `requirements.txt`, and `Dockerfile`. (pending)
- Normalize scan results to use only the `service` concept and never create a `frontend` versus `backend` split. (pending)

## Day 8 - Service Detection

Goal: turn scan results into meaningful service candidates for `init`.

- Implement service detection, unique name/path validation, start hint inference, and health hint inference. (pending)
- Add warnings for ambiguous detection without auto-classifying services as `frontend` or `backend`. (pending)

## Day 9 - Init Plan Models and Review UX

Goal: lock the local models and review screen before creating bindings or writing YAML.

- Lock the `InitPlan`, `ServiceCandidate`, `DependencyBindingDraft`, and `CompatibilityPolicyDraft` models. (pending)
- Build the scan result review screen before creating bindings or writing `lazyops.yaml`. (pending)

## Day 10 - Projects and Target Selection

Goal: move `init` onto the project and target selection flow by runtime mode.

- Implement fetch/list for projects and target summaries by runtime mode. (pending)
- Block targets that are incompatible with the selected mode and ensure target IPs or secrets never enter the repo contract. (pending)

## Day 11 - Binding Selection and Creation

Goal: allow `init` to reuse an existing binding or create a new one according to the spec.

- Implement the flow to choose an existing binding or create a new `DeploymentBinding`. (pending)
- Lock the bindings list contract to support both `init` and `lazyops bindings`. (pending)

## Day 12 - lazyops.yaml Schema Lock

Goal: lock the deploy contract schema before starting the generator.

- Lock the `lazyops.yaml` schema including `project_slug`, `runtime_mode`, `deployment_binding.target_ref`, `services`, `dependency_bindings`, and the policy blocks. (pending)
- Ban any field that carries secrets, raw infrastructure data, or direct deploy authority. (pending)

## Day 13 - YAML Generator

Goal: generate `lazyops.yaml` according to spec for all three runtime modes.

- Implement the YAML generator for `standalone`, `distributed-mesh`, and `distributed-k3s`. (pending)
- Apply spec-correct defaults for `env_injection`, `managed_credentials`, `localhost_rescue`, `sslip.io` or `nip.io`, and opt-in `scale-to-zero`. (pending)

## Day 14 - Validation and Safe Write Flow

Goal: validate the contract before writing the file and protect any existing file on disk.

- Integrate local schema validation or a backend validation route/package for `lazyops.yaml`. (pending)
- Implement pre-write review, overwrite confirmation, and backup strategy for `lazyops.yaml`. (pending)

## Day 15 - Standalone Init Happy Path

Goal: complete `lazyops init` for `standalone`.

- Complete the happy path for `lazyops init` in `standalone` mode. (pending)
- Write failure-path tests for no valid target and invalid PAT cases. (pending)

## Day 16 - Distributed Mesh Init

Goal: complete `init` for `distributed-mesh` with a multi-service graph.

- Complete `lazyops init` for `distributed-mesh` with multi-service handling and dependency binding review. (pending)
- Write failure-path tests for mesh ownership mismatch or offline targets. (pending)

## Day 17 - Distributed K3s Init

Goal: complete `init` for `distributed-k3s` without breaking the K3s boundary.

- Complete `lazyops init` for `distributed-k3s` without bypassing the K3s boundary. (pending)
- Write failure-path tests for cluster mismatch or unavailable cluster cases. (pending)

## Day 18 - Repo Link

Goal: connect the local repo to the project and GitHub App installation using the trust model in the spec.

- Implement `lazyops link` to connect the local repo to the project, GitHub App installation, and tracked branch. (pending)
- Verify repo ownership, project ownership, GitHub App installation, and that the target is online or at least registered. (pending)

## Day 19 - Bindings Command

Goal: treat bindings as a first-class command and let `init` reuse them.

- Implement `lazyops bindings` to list, filter, and reuse bindings by `target_ref`, runtime mode, target kind, and status. (pending)
- Ensure `init` can reuse a binding instead of always creating a new one. (pending)

## Day 20 - Doctor Command

Goal: create the health validation command for onboarding and deploy contract integrity.

- Implement `lazyops doctor` to validate auth, repo link, `lazyops.yaml`, binding existence, dependency declarations, and webhook health. (pending)
- Standardize `pass`, `warn`, and `fail` output along with the next fix step. (pending)

## Day 21 - Status Command

Goal: create a thin status aggregator without turning the CLI into a deploy engine.

- Implement `lazyops status` as a thin aggregator over deployment, topology, and binding state. (pending)
- Lock either a dedicated status contract or an adapter that composes status from the existing APIs. (pending)

## Day 22 - Logs Command

Goal: inspect service logs through a stable stream contract.

- Implement `lazyops logs <service>` via `GET /ws/logs/stream` with clean filtering and cancellation. (pending)
- Add secret redaction and error copy that clearly states which service, project, or filter is wrong. (pending)

## Day 23 - Traces Command

Goal: inspect request flow by correlation id.

- Implement `lazyops traces <correlation-id>` using `GET /api/v1/traces/:correlation_id`. (pending)
- Format the output to clearly show the service path, node hops, latency hotspots, and correlation id. (pending)

## Day 24 - Tunnel Abstraction

Goal: lock the session abstraction for the debug tunnel commands.

- Implement tunnel client/session abstraction for `tunnel db` and `tunnel tcp`. (pending)
- Lock timeout behavior, permission checks, and cleanup policy so tunnels remain debug tools and not production access paths. (pending)

## Day 25 - Tunnel DB

Goal: complete `lazyops tunnel db` with the full lifecycle.

- Complete `lazyops tunnel db` with port selection, conflict handling, stop, and cleanup. (pending)
- Write failure-path tests for offline targets, busy local ports, or permission denied. (pending)

## Day 26 - Tunnel TCP

Goal: complete `lazyops tunnel tcp` and keep the UX consistent.

- Complete `lazyops tunnel tcp` with UX consistent with `tunnel db`. (pending)
- Add a clear warning that this is a debug path, not a production access pattern. (pending)

## Day 27 - UX and Shell Polish

Goal: review the CLI experience so it feels Git-like and shell-friendly.

- Review all spinners, prompts, confirmations, help text, exit codes, and shell-friendly output. (pending)
- Ensure every destructive action requires confirmation and every error clearly states the next fix step. (pending)

## Day 28 - Security Hardening

Goal: lock the CLI security boundary before final verification.

- Harden security/config for the credential fallback file, PAT revoke path, and secret redaction. (pending)
- Verify that `lazyops.yaml` never contains SSH, kubeconfig, passwords, private keys, long-lived GitHub credentials, or server IP truth. (pending)

## Day 29 - Full Verification Matrix

Goal: validate the implementation against the acceptance cases in the master plan.

- Run the full unit, contract, and integration matrix for auth, init, link, doctor, bindings, status, logs, traces, and tunnels. (pending)
- Validate acceptance cases for revoked PAT, invalid `target_ref`, runtime mismatch, unlinked repo, and invalid dependency binding. (pending)

## Day 30 - Bug Bash and Release Readiness

Goal: close the roadmap with the release checklist and confirm the core command surface is complete.

- Run a bug bash, fix regression blockers, and close non-essential TODOs for v1. (pending)
- Finalize the release checklist, README, help, and examples, and only declare CLI v1 complete when the full command surface is in place. (pending)

## Test Plan

- Auth: wrong password is rejected, revoked PAT cannot be reused, and keychain fallback file handling remains safe.
- Init and yaml: scanning detects common signals correctly, the CLI never forces `frontend` or `backend`, `lazyops.yaml` is valid for all three runtime modes, and invalid `target_ref` plus runtime mismatch are blocked.
- Link and doctor: repo ownership, project ownership, GitHub App installation, binding existence, dependency binding validity, and webhook health are all validated correctly.
- Observability and debug: `status` reflects deployment/topology, `logs` stream is stable, `traces` shows the correct correlation path, and tunnel commands report the right failure when the target is offline or the port is busy.
- Security and UX: no secret appears in logs, output, or yaml; spinners appear for actions over one second; destructive actions require confirmation; every error points to the next fix step.

## Assumptions

- Day 1 through Day 30 in this file is a live tracker; every task starts in `(pending)` status.
- The roadmap assumes one main CLI workstream; if backend contracts are not ready, the CLI must first mock against the locked schema and later swap in the real contract without changing the public UX.
- Tunnel features are optional in the spec but remain in this tracker to reach a complete `CLI v1`; if the schedule slips, the core gate still prioritizes `login`, `logout`, `init`, `link`, `doctor`, `status`, `bindings`, `logs`, and `traces`.
