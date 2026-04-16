#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

NAMESPACE="${LAZYOPS_D910_NAMESPACE:-lazyops-day9-day10}"
APP_IMAGE="${LAZYOPS_D910_IMAGE:-nginx:1.27-alpine}"
APP_HOST="${LAZYOPS_D910_HOST:-public-smoke.127.0.0.1.nip.io}"

cp "${ROOT_DIR}/kustomization.yaml" "${TMP_DIR}/kustomization.yaml"
cp "${ROOT_DIR}/00-ingress-rollout-smoke.yaml" "${TMP_DIR}/00-ingress-rollout-smoke.yaml"

sed -i "s/lazyops-day9-day10/${NAMESPACE}/g" "${TMP_DIR}/00-ingress-rollout-smoke.yaml"
sed -i "s#nginx:1.27-alpine#${APP_IMAGE}#g" "${TMP_DIR}/00-ingress-rollout-smoke.yaml"
sed -i "s/public-smoke\\.127\\.0\\.0\\.1\\.nip\\.io/${APP_HOST}/g" "${TMP_DIR}/00-ingress-rollout-smoke.yaml"

kubectl apply -k "${TMP_DIR}"
kubectl -n "${NAMESPACE}" rollout status deployment/public-smoke --timeout=180s

echo "== Ingress =="
kubectl -n "${NAMESPACE}" get ingress public-smoke -o wide
echo
echo "== Ingress JSONPath =="
kubectl -n "${NAMESPACE}" get ingress public-smoke -o jsonpath='{.spec.rules[*].host}{"\n"}{.status.loadBalancer.ingress[*].ip}{"\n"}{.status.loadBalancer.ingress[*].hostname}{"\n"}'
echo
echo "== Traefik Service =="
kubectl -n kube-system get service traefik -o wide || true
