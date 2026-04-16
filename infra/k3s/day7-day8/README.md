# LazyOps K3s Day 7-8 DNS + Namespace Access Pack

Pack nay dong hai lane ha tang con mo cua Day 7 va Day 8 o muc repo artifact:

- Day 7: smoke test `nslookup` + `pg_isready` trong cung namespace de verify `Internal DNS` va private traffic
- Day 8: bo `ServiceAccount/Role/RoleBinding` namespace-scoped cho agent, kem token secret va kubeconfig template cho workflow ngoai cluster

## Thanh phan

- `00-namespace-agent-access.yaml`: namespace + `ServiceAccount` + `Role` + `RoleBinding` + token secret bootstrap
- `10-internal-dns-smoke.yaml`: postgres service + `dns-smoke` job + `pg-smoke` job
- `20-namespace-agent-kubeconfig.template.yaml`: template secret chua kubeconfig namespace-scoped
- `validate.sh`: harness patch namespace/image truoc khi `kubectl apply -k`

## Bien moi truong ho tro

- `LAZYOPS_D78_NAMESPACE`
- `LAZYOPS_D78_POSTGRES_IMAGE`
- `LAZYOPS_D78_DNS_IMAGE`

## Cach dung

1. Dam bao Day 1-6 da co san.
2. Chay:

```bash
bash infra/k3s/day7-day8/validate.sh
```

3. Kiem tra:

- `deployment/db-service` len `Ready`
- `job/dns-smoke` complete voi `nslookup db-service` va FQDN trong namespace
- `job/pg-smoke` complete voi `pg_isready -h db-service -p 5432`
- `ServiceAccount/Role/RoleBinding` `lazyops-namespace-agent` ton tai trong namespace

## Kubeconfig / Token strategy

- In-cluster agent uu tien `ServiceAccount` + projected token, namespace scope theo `RoleBinding`.
- Neu can agent ngoai cluster, dien token/CA/server that vao `20-namespace-agent-kubeconfig.template.yaml`.
- Secret token `lazyops-namespace-agent-token` duoc tao theo kieu `kubernetes.io/service-account-token`; tren cluster moi, co the thay bang `kubectl create token` neu policy cua cum khong populate secret tu dong.

## Ghi chu

- Day 7 va Day 8 duoc tick o muc repo artifact + harness. Verify tren cum K3s that van nam o lane integration sau.
