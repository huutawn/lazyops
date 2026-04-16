#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="${LAZYOPS_D78_NAMESPACE:-lazyops-day7-day8}"
POSTGRES_IMAGE="${LAZYOPS_D78_POSTGRES_IMAGE:-postgres:16-alpine}"
DNS_IMAGE="${LAZYOPS_D78_DNS_IMAGE:-busybox:1.36}"
WORKDIR="$(mktemp -d)"

cleanup() {
  rm -rf "${WORKDIR}"
}
trap cleanup EXIT

cp -R infra/k3s/day7-day8/. "${WORKDIR}/"
find "${WORKDIR}" -type f \( -name '*.yaml' -o -name '*.yml' \) -print0 | while IFS= read -r -d '' file; do
  sed -i \
    -e "s/lazyops-day7-day8/${NAMESPACE}/g" \
    -e "s#postgres:16-alpine#${POSTGRES_IMAGE}#g" \
    -e "s#busybox:1.36#${DNS_IMAGE}#g" \
    "${file}"
done

echo "==> applying day7-day8 dns + access pack"
kubectl apply -k "${WORKDIR}"

echo "==> waiting for postgres service"
kubectl -n "${NAMESPACE}" rollout status deployment/db-service --timeout=180s

echo "==> waiting for internal dns smoke jobs"
kubectl -n "${NAMESPACE}" wait --for=condition=complete job/dns-smoke --timeout=180s
kubectl -n "${NAMESPACE}" wait --for=condition=complete job/pg-smoke --timeout=180s

echo "==> printing namespace-scoped agent access resources"
kubectl -n "${NAMESPACE}" get serviceaccount lazyops-namespace-agent
kubectl -n "${NAMESPACE}" get role lazyops-namespace-agent
kubectl -n "${NAMESPACE}" get rolebinding lazyops-namespace-agent
kubectl -n "${NAMESPACE}" get secret lazyops-namespace-agent-token --ignore-not-found

echo "==> day7-day8 dns + access pack completed"
