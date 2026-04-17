#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

NAMESPACE="${LAZYOPS_D1315_NAMESPACE:-lazyops-day13-day15}"
DB_IMAGE="${LAZYOPS_D1315_DB_IMAGE:-postgres:16-alpine}"
API_IMAGE="${LAZYOPS_D1315_API_IMAGE:-busybox:1.36}"
WEB_IMAGE="${LAZYOPS_D1315_WEB_IMAGE:-nginx:1.27-alpine}"
APP_HOST="${LAZYOPS_D1315_HOST:-lazyops-e2e.127.0.0.1.nip.io}"
DATABASE_URL="${LAZYOPS_D1315_DATABASE_URL:-postgres://app:app-secret@db-service:5432/app}"
BAD_IMAGE="${LAZYOPS_D1315_BAD_IMAGE:-ghcr.io/lazyops/does-not-exist:broken}"

cp "${ROOT_DIR}/kustomization.yaml" "${TMP_DIR}/kustomization.yaml"
cp "${ROOT_DIR}/00-namespace.yaml" "${TMP_DIR}/00-namespace.yaml"
cp "${ROOT_DIR}/10-e2e-stack.yaml" "${TMP_DIR}/10-e2e-stack.yaml"

sed -i \
  -e "s/lazyops-day13-day15/${NAMESPACE}/g" \
  -e "s#postgres:16-alpine#${DB_IMAGE}#g" \
  -e "0,/busybox:1.36/s#busybox:1.36#${API_IMAGE}#" \
  -e "s#nginx:1.27-alpine#${WEB_IMAGE}#g" \
  -e "s/lazyops-e2e\\.127\\.0\\.0\\.1\\.nip\\.io/${APP_HOST}/g" \
  -e "s#postgres://app:app-secret@db-service:5432/app#${DATABASE_URL}#g" \
  "${TMP_DIR}/00-namespace.yaml" "${TMP_DIR}/10-e2e-stack.yaml"

echo "== Happy path =="
kubectl apply -k "${TMP_DIR}"
kubectl -n "${NAMESPACE}" rollout status deployment/db-service --timeout=240s
kubectl -n "${NAMESPACE}" rollout status deployment/api --timeout=240s
kubectl -n "${NAMESPACE}" rollout status deployment/web --timeout=240s
kubectl -n "${NAMESPACE}" get pods -L lazyops.service,lazyops.revision_id
kubectl -n "${NAMESPACE}" get svc
kubectl -n "${NAMESPACE}" get ingress web -o wide
kubectl -n "${NAMESPACE}" logs -l lazyops.service=api --tail=5 --timestamps

echo
echo "== Force bad rollout on web =="
kubectl -n "${NAMESPACE}" set image deployment/web web="${BAD_IMAGE}"
if kubectl -n "${NAMESPACE}" rollout status deployment/web --timeout=60s; then
  echo "expected bad web rollout to fail but it became ready"
  exit 1
fi
kubectl -n "${NAMESPACE}" get pods -l lazyops.service=web -o wide

echo
echo "== Re-apply stable manifest bundle =="
kubectl apply -k "${TMP_DIR}"
kubectl -n "${NAMESPACE}" rollout status deployment/web --timeout=240s
kubectl -n "${NAMESPACE}" get ingress web -o wide
kubectl -n "${NAMESPACE}" get pvc db-data
