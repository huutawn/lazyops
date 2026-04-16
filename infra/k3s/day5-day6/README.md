# LazyOps K3s Day 5-6 Platform Pack

Pack nay dong hai lane con lai cua Day 5 va Day 6 o muc repo artifact:

- Day 5 ha tang: image matrix cho `nginx`, `next-like`, `go`, `postgres`, `multi-port`
- Day 6 ha tang: chuan hoa `labels`, `annotations`, `ResourceQuota`, `LimitRange`

## Thanh phan

- `00-namespace-standards.yaml`: namespace labels/annotations + `ResourceQuota` + `LimitRange`
- `10-port-matrix.yaml`: workload matrix de check port defaults va multi-port behavior
- `validate.sh`: harness patch namespace/image truoc khi `kubectl apply -k`

## Bien moi truong ho tro

- `LAZYOPS_D56_NAMESPACE`
- `LAZYOPS_MATRIX_NGINX_IMAGE`
- `LAZYOPS_MATRIX_NEXT_IMAGE`
- `LAZYOPS_MATRIX_GO_IMAGE`
- `LAZYOPS_MATRIX_POSTGRES_IMAGE`
- `LAZYOPS_MATRIX_MULTIPORT_IMAGE`

## Cach dung

1. Dam bao Day 1-4 da co san.
2. Chay:

```bash
bash infra/k3s/day5-day6/validate.sh
```

3. Kiem tra:

- `ResourceQuota` va `LimitRange` ton tai trong namespace
- ca 5 workload matrix len `Ready`
- `matrix-multiport` expose duoc 2 ports tren Service

## Ghi chu

- `matrix-next` dung `node:20-alpine` de mo phong app Next default port `3000`.
- `matrix-go` dung `traefik/whoami`, la binary Go HTTP nhe va on dinh cho smoke pack.
- Pack nay la harness repo-level; verify tren cum K3s that van thuoc lane integration sau sprint.
