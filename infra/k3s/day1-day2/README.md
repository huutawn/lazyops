# LazyOps K3s Day 1-2 Bootstrap

Muc tieu cua bo artifact nay:

- chot bootstrap cum K3s cho `distributed-k3s`
- giu `Traefik` la ingress mac dinh
- co `namespace bootstrap`, `RBAC`, `registry pull secret`, `node-agent DaemonSet`
- co preflight de check node, Traefik, storage class, registry reachability

## Apply order

1. `kubectl apply -f infra/k3s/day1-day2/00-lazyops-system.yaml`
2. Dien gia tri that vao:
   - `infra/k3s/day1-day2/03-node-agent-secret.template.yaml`
   - `infra/k3s/day1-day2/04-registry-secret.template.yaml`
3. `kubectl apply -f infra/k3s/day1-day2/01-traefik-helmchartconfig.yaml`
4. `kubectl apply -f infra/k3s/day1-day2/02-node-agent-rbac.yaml`
5. `kubectl apply -f infra/k3s/day1-day2/03-node-agent-secret.template.yaml`
6. `kubectl apply -f infra/k3s/day1-day2/04-registry-secret.template.yaml`
7. `kubectl apply -f infra/k3s/day1-day2/05-node-agent-daemonset.yaml`
8. `bash infra/k3s/day1-day2/preflight.sh`

## Kubeconfig strategy

- Backend chi luu `cluster.kubeconfig_secret_ref`, khong luu raw kubeconfig trong repo contract.
- Operator bootstrap bang `kubectl` local va `KUBECONFIG` cua minh.
- Node agent trong cluster dung `ServiceAccount` + in-cluster auth; khong can mount kubeconfig that vao container.
- Neu can workflow ngoai cluster, secret ref phai tro ve secret quan ly boi platform, khong ghi kubeconfig vao `lazyops.yaml`.

## Registry strategy

- `lazyops-registry` la `dockerconfigjson` dung chung trong `lazyops-system`.
- K3s project namespace moi can copy/pull secret nay khi backend sinh namespace/service cho project.
- Neu su dung private registry, bo sung `imagePullSecrets` trong namespace/project overlay.

## Notes

- `tawn/lazyops-agent:latest` hien da can co `kubectl` trong image de `node_agent` apply manifest.
- Preflight script co the nhan them:
  - `LAZYOPS_REGISTRY_SMOKE_IMAGE`
  - `LAZYOPS_REGISTRY_PULL_SECRET`
  - `LAZYOPS_PREFLIGHT_NAMESPACE`
