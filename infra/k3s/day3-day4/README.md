# LazyOps K3s Day 3-4 Smoke Pack

Bo artifact nay phuc vu hai muc trong sprint:

- Day 3 ha tang: verify `PVC dynamic provisioning`, `image pull`, `pod scheduling`
- Day 4 ha tang: smoke deploy image mau de check `networking`, `secret mount`, `Traefik ingress`

## Cach dung

1. Dam bao da apply bo Day 1-2 truoc.
2. Chinh lai bien moi truong neu can:
   - `LAZYOPS_SMOKE_APP_IMAGE`
   - `LAZYOPS_SMOKE_REGISTRY_IMAGE`
   - `LAZYOPS_SMOKE_REGISTRY_SECRET`
   - `LAZYOPS_SMOKE_NAMESPACE`
   - `LAZYOPS_SMOKE_APP_HOST`
3. Chay:

```bash
bash infra/k3s/day3-day4/smoke.sh
```

4. Neu can cleanup:

```bash
kubectl delete namespace lazyops-smoke --ignore-not-found
```

## Ket qua mong doi

- `pvc-smoke` vao `Bound`
- `registry-smoke` pull duoc image va len `Ready`
- `scheduler-smoke` tao du replica va co the quan sat node placement
- `app-smoke` len `Ready`, mount secret thanh cong
- host trong Ingress duoc patch theo `LAZYOPS_SMOKE_APP_HOST`

## Ghi chu

- Pack nay la smoke/integration artifact, khong phai benchmark hoac soak test.
- Script se render mot workspace tam de patch namespace, image, registry secret va host truoc khi `kubectl apply -k`.
- Neu cluster dung private registry, nho dien `imagePullSecrets`.
