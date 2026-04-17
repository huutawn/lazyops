#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

NAMESPACE="${LAZYOPS_D1315_NAMESPACE:-lazyops-day13-day15}"
DB_IMAGE="${LAZYOPS_D1315_DB_IMAGE:-postgres:16-alpine}"
API_IMAGE="${LAZYOPS_D1315_API_IMAGE:-busybox:1.36}"
WEB_IMAGE="${LAZYOPS_D1315_WEB_IMAGE:-nginx:1.27-alpine}"
TOOLS_IMAGE="${LAZYOPS_D1315_TOOLS_IMAGE:-busybox:1.36}"
APP_HOST="${LAZYOPS_D1315_HOST:-lazyops-e2e.127.0.0.1.nip.io}"
DATABASE_URL="${LAZYOPS_D1315_DATABASE_URL:-postgres://app:app-secret@db-service:5432/app}"

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

kubectl apply -k "${TMP_DIR}"
kubectl -n "${NAMESPACE}" rollout status deployment/db-service --timeout=240s
kubectl -n "${NAMESPACE}" rollout status deployment/api --timeout=240s
kubectl -n "${NAMESPACE}" rollout status deployment/web --timeout=240s

echo "== Verify api -> db-service internal DNS =="
kubectl -n "${NAMESPACE}" exec deployment/api -- sh -c 'nc -z db-service 5432'

echo "== Verify in-cluster http path =="
kubectl -n "${NAMESPACE}" run http-smoke \
  --rm -i --restart=Never \
  --image="${TOOLS_IMAGE}" \
  --command -- sh -c 'wget -qO- http://api:8080/ && echo && wget -qO- http://web/ && echo'

echo "== Ingress =="
kubectl -n "${NAMESPACE}" get ingress web -o wide
echo
echo "== Traefik Service =="
kubectl -n kube-system get service traefik -o wide || true
echo
echo "== PVC =="
kubectl -n "${NAMESPACE}" get pvc db-data
