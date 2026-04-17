#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="${LAZYOPS_D1315_NAMESPACE:-lazyops-day13-day15}"
CYCLES="${LAZYOPS_D1315_CYCLES:-3}"
SLEEP_SECONDS="${LAZYOPS_D1315_SLEEP_SECONDS:-10}"

if [[ "${CYCLES}" -lt 1 ]]; then
  echo "LAZYOPS_D1315_CYCLES must be >= 1"
  exit 1
fi

for cycle in $(seq 1 "${CYCLES}"); do
  echo "== Burn-in cycle ${cycle}/${CYCLES} =="

  kubectl -n "${NAMESPACE}" rollout restart deployment/api deployment/web
  kubectl -n "${NAMESPACE}" rollout status deployment/api --timeout=240s
  kubectl -n "${NAMESPACE}" rollout status deployment/web --timeout=240s

  API_POD="$(kubectl -n "${NAMESPACE}" get pods -l app.kubernetes.io/name=api -o jsonpath='{.items[0].metadata.name}')"
  if [[ -n "${API_POD}" ]]; then
    echo "== Force api pod reschedule =="
    kubectl -n "${NAMESPACE}" delete pod "${API_POD}" --wait=true
    kubectl -n "${NAMESPACE}" rollout status deployment/api --timeout=240s
  fi

  echo "== Dependency check =="
  kubectl -n "${NAMESPACE}" exec deployment/api -- sh -c 'nc -z db-service 5432'

  echo "== Recent api logs =="
  kubectl -n "${NAMESPACE}" logs -l lazyops.service=api --tail=10 --timestamps
  echo
  echo "== Recent web logs =="
  kubectl -n "${NAMESPACE}" logs -l lazyops.service=web --tail=10 --timestamps
  echo
  echo "== Pod inventory =="
  kubectl -n "${NAMESPACE}" get pods -L lazyops.service,lazyops.revision_id

  sleep "${SLEEP_SECONDS}"
done
