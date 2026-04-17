# Day 13-15 K3s Integration Pack

Bo artifact nay dong vai tro repo harness cho 3 ngay cuoi cua sprint:

- Day 13: stack `web/api/db` de verify internal DNS, public ingress, rollout readiness, va PVC cho DB
- Day 14: burn-in/restart soak de bat loi rollout/log-stream/networking edge case truoc demo
- Day 15: demo flow + release checklist + script upgrade node-agent DaemonSet

## Files

- `00-namespace.yaml`: namespace cho integration pack
- `10-e2e-stack.yaml`: stack `db-service + api + web + ingress + pvc`
- `kustomization.yaml`: kustomize entrypoint
- `integration-e2e.sh`: apply stack, wait rollout, verify `api -> db-service:5432`, verify `web -> api`
- `burn-in.sh`: rollout restart + pod reschedule loop de soak stack va log labels
- `demo-flow.sh`: flow demo `happy path -> bad rollout -> re-apply stable manifest`
- `upgrade-node-agent.sh`: upgrade image cho `lazyops-node-agent` DaemonSet

## Env vars

- `LAZYOPS_D1315_NAMESPACE`: namespace test, mac dinh `lazyops-day13-day15`
- `LAZYOPS_D1315_DB_IMAGE`: image PostgreSQL, mac dinh `postgres:16-alpine`
- `LAZYOPS_D1315_API_IMAGE`: image API smoke, mac dinh `busybox:1.36`
- `LAZYOPS_D1315_WEB_IMAGE`: image web smoke, mac dinh `nginx:1.27-alpine`
- `LAZYOPS_D1315_TOOLS_IMAGE`: image helper cho smoke command, mac dinh `busybox:1.36`
- `LAZYOPS_D1315_HOST`: ingress host, mac dinh `lazyops-e2e.127.0.0.1.nip.io`
- `LAZYOPS_D1315_DATABASE_URL`: DSN noi bo, mac dinh `postgres://app:app-secret@db-service:5432/app`
- `LAZYOPS_D1315_CYCLES`: so vong burn-in, mac dinh `3`
- `LAZYOPS_D1315_BAD_IMAGE`: image loi cho demo rollback, mac dinh `ghcr.io/lazyops/does-not-exist:broken`

## Typical flow

```bash
infra/k3s/day13-day15/integration-e2e.sh
infra/k3s/day13-day15/burn-in.sh
infra/k3s/day13-day15/demo-flow.sh
infra/k3s/day13-day15/upgrade-node-agent.sh
```

## Scope

- Pack nay dong muc `repo artifact + script harness + release docs`.
- Ban van can cum K3s that de chay E2E API/build/push that, verify reachability tu internet, va chap nhan go-live.
