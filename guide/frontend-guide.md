# Frontend Guide

## 1. Role

Frontend is the operator experience layer for LazyOps. It should make one-server and distributed deployments feel like the same product, with extra topology tooling appearing only when needed.

Frontend must support:

- onboarding and auth
- instance, mesh, and cluster management
- project and service management
- GitHub App integration
- deployment history and rollout state
- topology visualization and editing
- logs, traces, incidents, and FinOps views

## 2. Product UX Principles

- the user thinks in `services`, not frontend/backend categories
- the user should not need to read infrastructure YAML
- onboarding should lead the user from login to deployable project quickly
- distributed complexity should be visual, not text-heavy
- topology editing should use drag and drop when the target mode is distributed

## 3. Recommended Frontend Stack

- React
- TypeScript
- Vite or Next.js app shell
- TanStack Query for server state
- Zustand for local UI state
- React Flow for topology canvas
- xterm.js for terminal and log views if needed
- a component library chosen for speed and consistency

## 4. Main Screens

### 4.1 Auth

- email and password login
- Google OAuth2 login
- GitHub OAuth2 login
- post-login session restore

### 4.2 Onboarding

- create project
- connect GitHub App
- register first instance
- explain runtime modes:
  - standalone
  - distributed-mesh
  - distributed-k3s

### 4.3 Project Dashboard

- latest deployment state
- service list
- health summary
- recent incidents
- quick links to logs and traces

### 4.4 Targets

- instances list
- mesh networks list
- clusters list
- online and offline status
- bootstrap command modal for new targets

### 4.5 Repository and Integration

- GitHub App installation status
- linked repository
- tracked branches
- webhook health
- recent build runs

### 4.6 Topology Canvas

Use React Flow to show:

- instances or cluster nodes
- services running on targets
- service-to-service edges
- health and latency on edges
- degraded or offline targets highlighted clearly

In distributed modes the user should be able to:

- drag services between nodes
- connect service dependencies visually
- inspect sidecar or mesh route details

### 4.7 Deployments

- release history
- build status
- rollout timeline
- zero-downtime promotion state
- rollback actions

### 4.8 Observability

- live logs
- traces by `X-Correlation-ID`
- incident feed
- bottleneck and latency view
- topology-linked drilldown

### 4.9 FinOps

- per-service CPU and RAM trends
- edge-downsampled `p95`, `max`, `min`, `avg`
- idle candidates for scale-to-zero
- cost advisory

## 5. Runtime Mode UX

### 5.1 Standalone

Frontend should simplify the UI:

- one instance focus
- topology optional
- deployment and logs prioritized

### 5.2 Distributed Mesh

Frontend should emphasize:

- multi-instance topology
- sidecar and mesh links
- placement and dependency edges

### 5.3 Distributed K3s

Frontend should add:

- cluster health
- node status
- workload placement summaries
- Kubernetes-backed rollouts without exposing raw K8s YAML as the main UX

## 6. Auth and Identity UX

### 6.1 OAuth screens

Frontend must provide:

- Google login button
- GitHub login button
- email/password fallback

### 6.2 Session model

- web app uses JWT or secure session cookies as defined by backend
- CLI identity is separate and uses PAT

### 6.3 GitHub App onboarding

The UI should guide the user through:

1. install LazyOps GitHub App
2. choose repositories
3. sync installations
4. link repository to project
5. confirm tracked branches and webhook state

## 7. Init and Project Contract UX

Frontend should mirror what CLI does so both can be built in parallel.

Needed UI flows:

- inspect generated service list
- choose runtime mode
- select target binding
- review generated deploy contract
- review compatibility settings
- review health check and domain policy

Even if CLI remains the main place that writes `lazyops.yaml`, frontend should still expose and explain the same model.

## 8. API Contracts Frontend Needs Early

### 8.1 Auth

- login
- register
- current user
- Google and GitHub OAuth start URLs

### 8.2 Targets

- instances list and detail
- mesh networks list and detail
- clusters list and detail
- bootstrap command issue

### 8.3 Projects

- projects list and detail
- services list
- deployment bindings
- blueprint summary

### 8.4 Integration

- GitHub installations
- repositories
- repo link status
- webhook health

### 8.5 Deployment and runtime

- deployments list
- release detail
- topology graph
- logs stream
- traces detail
- metrics aggregates

## 9. Mock-First Delivery Plan

Frontend must be able to start before runtime is complete.

Recommended order:

1. mock auth
2. mock targets and project list
3. mock GitHub App integration state
4. mock topology graph
5. mock deployments and trace detail
6. swap each module to real APIs gradually

Use:

- shared TypeScript schemas
- mock service worker or local fixture layer
- feature flags for incomplete pages

## 10. Component Boundaries

### 10.1 Shell

- app layout
- navigation
- session guard

### 10.2 Feature modules

- `auth`
- `projects`
- `targets`
- `integrations`
- `topology`
- `deployments`
- `observability`
- `finops`

### 10.3 Shared primitives

- status badge
- health chip
- timeline
- node card
- edge inspector
- log viewer
- trace view

## 11. React Flow Design Notes

The topology page should render:

- target nodes
- service nodes
- dependency edges
- health and latency badges
- alert overlays

Expected interactions:

- click service to open detail drawer
- click edge to inspect route, sidecar, and latency
- drag service to another target in planning mode
- compare current topology vs desired topology

## 12. Parallel Work Plan

### 12.1 Frontend tracks

- auth and onboarding
- project dashboard
- targets management
- GitHub integration
- topology canvas
- deployments and logs
- traces and incidents
- FinOps dashboards

### 12.2 Shared contracts to lock early

- auth response
- instance, mesh, and cluster summary
- project and service summary
- deployment binding summary
- topology node and edge payload
- trace summary payload
- metrics aggregate payload

## 13. Definition of Done for Frontend

Frontend is ready for first integrated release when:

- a user can log in with email, Google, or GitHub
- a user can see and register targets
- a user can connect GitHub App and link a repo
- a user can inspect service topology in React Flow
- a user can view deployments, logs, traces, and FinOps summaries
- the app works for both standalone and distributed modes without switching to a different product
