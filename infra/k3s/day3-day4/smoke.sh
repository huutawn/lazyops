#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="${LAZYOPS_SMOKE_NAMESPACE:-lazyops-smoke}"
REGISTRY_SECRET="${LAZYOPS_SMOKE_REGISTRY_SECRET:-lazyops-registry}"
APP_IMAGE="${LAZYOPS_SMOKE_APP_IMAGE:-nginx:1.27-alpine}"
REGISTRY_IMAGE="${LAZYOPS_SMOKE_REGISTRY_IMAGE:-busybox:1.36}"
APP_HOST="${LAZYOPS_SMOKE_APP_HOST:-app-smoke.127.0.0.1.nip.io}"
WORKDIR="$(mktemp -d)"

cleanup() {
  rm -rf "${WORKDIR}"
}
trap cleanup EXIT

cp -R infra/k3s/day3-day4/. "${WORKDIR}/"
find "${WORKDIR}" -type f \( -name '*.yaml' -o -name '*.yml' \) -print0 | while IFS= read -r -d '' file; do
  sed -i \
    -e "s/lazyops-smoke/${NAMESPACE}/g" \
    -e "s/lazyops-registry/${REGISTRY_SECRET}/g" \
    -e "s#nginx:1.27-alpine#${APP_IMAGE}#g" \
    -e "s#busybox:1.36#${REGISTRY_IMAGE}#g" \
    -e "s/app-smoke\\.127\\.0\\.0\\.1\\.nip\\.io/${APP_HOST}/g" \
    "${file}"
done

echo "==> applying smoke manifests"
kubectl apply -k "${WORKDIR}"

echo "==> waiting for registry pull smoke"
kubectl -n "${NAMESPACE}" rollout status deployment/registry-smoke --timeout=180s

echo "==> waiting for pvc smoke"
kubectl -n "${NAMESPACE}" wait --for=jsonpath='{.status.phase}'=Bound pvc/pvc-smoke --timeout=180s
kubectl -n "${NAMESPACE}" rollout status deployment/pvc-smoke --timeout=180s
PVC_POD="$(kubectl -n "${NAMESPACE}" get pod -l app.kubernetes.io/name=pvc-smoke -o jsonpath='{.items[0].metadata.name}')"
kubectl -n "${NAMESPACE}" exec "${PVC_POD}" -- cat /data/ready.txt

echo "==> waiting for scheduler smoke"
kubectl -n "${NAMESPACE}" rollout status deployment/scheduler-smoke --timeout=180s
kubectl -n "${NAMESPACE}" get pods -l app.kubernetes.io/name=scheduler-smoke -o wide

echo "==> waiting for app smoke"
kubectl -n "${NAMESPACE}" rollout status deployment/app-smoke --timeout=180s
kubectl -n "${NAMESPACE}" get ingress app-smoke
kubectl -n "${NAMESPACE}" get svc app-smoke

echo "==> smoke pack completed"
