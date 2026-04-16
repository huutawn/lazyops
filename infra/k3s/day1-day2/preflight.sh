#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="${LAZYOPS_PREFLIGHT_NAMESPACE:-lazyops-system}"
REGISTRY_SECRET="${LAZYOPS_REGISTRY_PULL_SECRET:-lazyops-registry}"
SMOKE_IMAGE="${LAZYOPS_REGISTRY_SMOKE_IMAGE:-busybox:1.36}"
SMOKE_POD="lazyops-registry-smoke"

echo "==> checking cluster access"
kubectl version --short >/dev/null

echo "==> checking nodes"
kubectl get nodes

echo "==> checking traefik"
kubectl -n kube-system get deployment traefik
kubectl -n kube-system get svc traefik

echo "==> checking default storageclass"
kubectl get storageclass
kubectl get storageclass -o jsonpath='{range .items[*]}{.metadata.name}{"="}{.metadata.annotations.storageclass\.kubernetes\.io/is-default-class}{"\n"}{end}' | grep "=true" >/dev/null

echo "==> checking lazyops namespace"
kubectl get namespace "${NAMESPACE}" >/dev/null

echo "==> checking registry secret"
kubectl -n "${NAMESPACE}" get secret "${REGISTRY_SECRET}" >/dev/null

echo "==> running registry pull smoke"
kubectl -n "${NAMESPACE}" delete pod "${SMOKE_POD}" --ignore-not-found >/dev/null 2>&1 || true
kubectl -n "${NAMESPACE}" run "${SMOKE_POD}" \
  --image="${SMOKE_IMAGE}" \
  --restart=Never \
  --overrides="{\"spec\":{\"imagePullSecrets\":[{\"name\":\"${REGISTRY_SECRET}\"}]}}" \
  --command -- sh -c 'echo ok'
kubectl -n "${NAMESPACE}" wait --for=condition=Ready "pod/${SMOKE_POD}" --timeout=120s
kubectl -n "${NAMESPACE}" logs "${SMOKE_POD}"
kubectl -n "${NAMESPACE}" delete pod "${SMOKE_POD}" --ignore-not-found >/dev/null 2>&1 || true

echo "==> preflight passed"
