#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="${LAZYOPS_D56_NAMESPACE:-lazyops-day5-day6}"
NGINX_IMAGE="${LAZYOPS_MATRIX_NGINX_IMAGE:-nginx:1.27-alpine}"
NEXT_IMAGE="${LAZYOPS_MATRIX_NEXT_IMAGE:-node:20-alpine}"
GO_IMAGE="${LAZYOPS_MATRIX_GO_IMAGE:-traefik/whoami:v1.10}"
POSTGRES_IMAGE="${LAZYOPS_MATRIX_POSTGRES_IMAGE:-postgres:16-alpine}"
MULTIPORT_IMAGE="${LAZYOPS_MATRIX_MULTIPORT_IMAGE:-mendhak/http-https-echo:31}"
WORKDIR="$(mktemp -d)"

cleanup() {
  rm -rf "${WORKDIR}"
}
trap cleanup EXIT

cp -R infra/k3s/day5-day6/. "${WORKDIR}/"
find "${WORKDIR}" -type f \( -name '*.yaml' -o -name '*.yml' \) -print0 | while IFS= read -r -d '' file; do
  sed -i \
    -e "s/lazyops-day5-day6/${NAMESPACE}/g" \
    -e "s#nginx:1.27-alpine#${NGINX_IMAGE}#g" \
    -e "s#node:20-alpine#${NEXT_IMAGE}#g" \
    -e "s#traefik/whoami:v1.10#${GO_IMAGE}#g" \
    -e "s#postgres:16-alpine#${POSTGRES_IMAGE}#g" \
    -e "s#mendhak/http-https-echo:31#${MULTIPORT_IMAGE}#g" \
    "${file}"
done

echo "==> applying day5-day6 platform pack"
kubectl apply -k "${WORKDIR}"

echo "==> checking namespace defaults"
kubectl -n "${NAMESPACE}" get resourcequota lazyops-defaults
kubectl -n "${NAMESPACE}" get limitrange lazyops-defaults

echo "==> waiting for matrix deployments"
kubectl -n "${NAMESPACE}" rollout status deployment/matrix-nginx --timeout=180s
kubectl -n "${NAMESPACE}" rollout status deployment/matrix-next --timeout=180s
kubectl -n "${NAMESPACE}" rollout status deployment/matrix-go --timeout=180s
kubectl -n "${NAMESPACE}" rollout status deployment/matrix-postgres --timeout=180s
kubectl -n "${NAMESPACE}" rollout status deployment/matrix-multiport --timeout=180s

echo "==> printing service ports"
kubectl -n "${NAMESPACE}" get svc matrix-nginx matrix-next matrix-go matrix-postgres matrix-multiport

echo "==> day5-day6 platform pack completed"
