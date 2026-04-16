# Day 9-10 K3s Repo Harness

Bo artifact nay dong vai tro harness repo-level cho 2 muc cua sprint:

- Day 9: verify `Traefik ingress readiness`, host routing, va external address observation
- Day 10: verify `rollback smoke` theo kieu reapplied stable manifest sau khi rollout loi

## Files

- `00-ingress-rollout-smoke.yaml`: namespace + deployment/service/ingress smoke cho app public
- `kustomization.yaml`: kustomize entrypoint
- `validate-ingress.sh`: apply base smoke va in ra `Ingress / Traefik service / host`
- `rollback-smoke.sh`: deploy ban on dinh, co tinh patch image loi, sau do re-apply stable manifest va cho rollout quay lai

## Env vars

- `LAZYOPS_D910_NAMESPACE`: namespace test, mac dinh `lazyops-day9-day10`
- `LAZYOPS_D910_IMAGE`: image on dinh, mac dinh `nginx:1.27-alpine`
- `LAZYOPS_D910_HOST`: host public, mac dinh `public-smoke.127.0.0.1.nip.io`
- `LAZYOPS_D910_BAD_IMAGE`: image loi de test rollback, mac dinh `ghcr.io/lazyops/does-not-exist:broken`

## Typical flow

```bash
infra/k3s/day9-day10/validate-ingress.sh
infra/k3s/day9-day10/rollback-smoke.sh
```

## Scope

- Muc tieu cua pack nay la repo artifact + script harness.
- Ban van can mot cum K3s that de xac nhan external reachability tu internet va rollback end-to-end.
