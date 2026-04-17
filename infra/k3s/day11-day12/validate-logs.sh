#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

NAMESPACE="${LAZYOPS_D1112_NAMESPACE:-lazyops-day11-day12}"
LOG_IMAGE="${LAZYOPS_D1112_LOG_IMAGE:-busybox:1.36}"

cp "${ROOT_DIR}/00-namespace.yaml" "${TMP_DIR}/00-namespace.yaml"
cp "${ROOT_DIR}/10-log-stream-smoke.yaml" "${TMP_DIR}/10-log-stream-smoke.yaml"

sed -i \
  -e "s/lazyops-day11-day12/${NAMESPACE}/g" \
  -e "s#busybox:1.36#${LOG_IMAGE}#g" \
  "${TMP_DIR}/00-namespace.yaml" "${TMP_DIR}/10-log-stream-smoke.yaml"

kubectl apply -f "${TMP_DIR}/00-namespace.yaml"
kubectl apply -f "${TMP_DIR}/10-log-stream-smoke.yaml"
kubectl -n "${NAMESPACE}" rollout status deployment/live-log-smoke --timeout=180s

sleep 5

echo "== Pods with lazyops labels =="
kubectl -n "${NAMESPACE}" get pods -l lazyops.service=live-log-smoke --show-labels
echo
echo "== Recent logs by service label =="
kubectl -n "${NAMESPACE}" logs -l lazyops.service=live-log-smoke --tail=20 --timestamps
echo
echo "== Recent logs by project label =="
kubectl -n "${NAMESPACE}" logs -l lazyops.project_id=prj-day11-day12 --tail=10 --timestamps
