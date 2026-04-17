# Day 11-12 K3s Repo Harness

Bo artifact nay dong vai tro harness repo-level cho 2 muc cua sprint:

- Day 11: verify `live log stream`, label-based log filter, va replay log sau khi pod thay doi
- Day 12: verify cac fail-path co cau truc cho `image pull fail`, `scheduling blocked`, `PVC pending`, `DNS fail`, `ingress backend not ready`

## Files

- `00-namespace.yaml`: namespace bootstrap cho pack day11-day12
- `10-log-stream-smoke.yaml`: deployment sinh log lien tuc + labels `lazyops.project_id / lazyops.revision_id / lazyops.service`
- `20-fail-path-smoke.yaml`: cac manifest smoke cho fail-path cua Day 12
- `kustomization.yaml`: kustomize entrypoint
- `validate-logs.sh`: apply log-smoke pack va in log/filter theo labels
- `fail-paths.sh`: apply fail-path pack va in ra cac trang thai loi can quan sat

## Env vars

- `LAZYOPS_D1112_NAMESPACE`: namespace test, mac dinh `lazyops-day11-day12`
- `LAZYOPS_D1112_LOG_IMAGE`: image cho log smoke, mac dinh `busybox:1.36`
- `LAZYOPS_D1112_DNS_IMAGE`: image cho DNS fail smoke, mac dinh `busybox:1.36`
- `LAZYOPS_D1112_APP_IMAGE`: image cho pending scheduler smoke, mac dinh `nginx:1.27-alpine`
- `LAZYOPS_D1112_BAD_IMAGE`: image loi cho `ImagePullBackOff`, mac dinh `ghcr.io/lazyops/does-not-exist:broken`
- `LAZYOPS_D1112_PENDING_STORAGE_CLASS`: storage class khong ton tai de tao `PVC Pending`, mac dinh `lazyops-does-not-exist`
- `LAZYOPS_D1112_FAIL_HOST`: host smoke cho ingress backend not ready, mac dinh `fail-smoke.127.0.0.1.nip.io`

## Typical flow

```bash
infra/k3s/day11-day12/validate-logs.sh
infra/k3s/day11-day12/fail-paths.sh
```

## Scope

- Pack nay dong muc `repo artifact + script harness`.
- Ban van can cum K3s that de soak test log resume, external ingress reachability, va fail-path tu ha tang that trong 3 ngay integration cuoi.
