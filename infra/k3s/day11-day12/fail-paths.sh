#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

NAMESPACE="${LAZYOPS_D1112_NAMESPACE:-lazyops-day11-day12}"
DNS_IMAGE="${LAZYOPS_D1112_DNS_IMAGE:-busybox:1.36}"
APP_IMAGE="${LAZYOPS_D1112_APP_IMAGE:-nginx:1.27-alpine}"
BAD_IMAGE="${LAZYOPS_D1112_BAD_IMAGE:-ghcr.io/lazyops/does-not-exist:broken}"
PENDING_STORAGE_CLASS="${LAZYOPS_D1112_PENDING_STORAGE_CLASS:-lazyops-does-not-exist}"
FAIL_HOST="${LAZYOPS_D1112_FAIL_HOST:-fail-smoke.127.0.0.1.nip.io}"

cp "${ROOT_DIR}/00-namespace.yaml" "${TMP_DIR}/00-namespace.yaml"
cp "${ROOT_DIR}/20-fail-path-smoke.yaml" "${TMP_DIR}/20-fail-path-smoke.yaml"

sed -i \
  -e "s/lazyops-day11-day12/${NAMESPACE}/g" \
  -e "s#busybox:1.36#${DNS_IMAGE}#g" \
  -e "s#nginx:1.27-alpine#${APP_IMAGE}#g" \
  -e "s#ghcr.io/lazyops/does-not-exist:broken#${BAD_IMAGE}#g" \
  -e "s/lazyops-does-not-exist/${PENDING_STORAGE_CLASS}/g" \
  -e "s/fail-smoke\\.127\\.0\\.0\\.1\\.nip\\.io/${FAIL_HOST}/g" \
  "${TMP_DIR}/00-namespace.yaml" "${TMP_DIR}/20-fail-path-smoke.yaml"

kubectl apply -f "${TMP_DIR}/00-namespace.yaml"
kubectl apply -f "${TMP_DIR}/20-fail-path-smoke.yaml"

echo "== Image pull fail smoke =="
if kubectl -n "${NAMESPACE}" rollout status deployment/image-pull-smoke --timeout=45s; then
  echo "expected image-pull-smoke rollout to fail but it became ready"
  exit 1
fi
kubectl -n "${NAMESPACE}" get pods -l lazyops.service=image-pull-smoke -o jsonpath='{range .items[*]}{.metadata.name}{" "}{.status.containerStatuses[0].state.waiting.reason}{"\n"}{end}' || true
echo
echo "== Scheduling blocked smoke =="
if kubectl -n "${NAMESPACE}" rollout status deployment/schedule-blocked-smoke --timeout=45s; then
  echo "expected schedule-blocked-smoke rollout to stay pending but it became ready"
  exit 1
fi
kubectl -n "${NAMESPACE}" get pods -l lazyops.service=schedule-blocked-smoke -o wide
echo
echo "== PVC pending smoke =="
kubectl -n "${NAMESPACE}" get pvc pvc-pending-smoke -o jsonpath='{.metadata.name}{" phase="}{.status.phase}{" storageClass="}{.spec.storageClassName}{"\n"}'
echo
echo "== DNS fail smoke =="
kubectl -n "${NAMESPACE}" wait --for=condition=failed job/dns-fail-smoke --timeout=60s || true
kubectl -n "${NAMESPACE}" get job dns-fail-smoke -o jsonpath='{.metadata.name}{" failed="}{.status.failed}{" succeeded="}{.status.succeeded}{"\n"}'
DNS_POD="$(kubectl -n "${NAMESPACE}" get pods -l job-name=dns-fail-smoke -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)"
if [[ -n "${DNS_POD}" ]]; then
  kubectl -n "${NAMESPACE}" logs "${DNS_POD}" || true
fi
echo
echo "== Ingress backend not ready smoke =="
kubectl -n "${NAMESPACE}" get ingress ingress-not-ready-smoke -o wide
kubectl -n "${NAMESPACE}" get endpoints ingress-backend-smoke -o wide
