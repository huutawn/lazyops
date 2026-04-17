#!/usr/bin/env bash
set -euo pipefail

AGENT_NAMESPACE="${LAZYOPS_AGENT_NAMESPACE:-lazyops-system}"
DAEMONSET_NAME="${LAZYOPS_AGENT_DAEMONSET:-lazyops-node-agent}"
CONTAINER_NAME="${LAZYOPS_AGENT_CONTAINER:-agent}"
AGENT_IMAGE="${LAZYOPS_AGENT_IMAGE:-tawn/lazyops-agent:latest}"
ROLLOUT_TIMEOUT="${LAZYOPS_AGENT_ROLLOUT_TIMEOUT:-300s}"

kubectl -n "${AGENT_NAMESPACE}" get daemonset "${DAEMONSET_NAME}" >/dev/null

kubectl -n "${AGENT_NAMESPACE}" set image "daemonset/${DAEMONSET_NAME}" "${CONTAINER_NAME}=${AGENT_IMAGE}"
kubectl -n "${AGENT_NAMESPACE}" rollout status "daemonset/${DAEMONSET_NAME}" --timeout="${ROLLOUT_TIMEOUT}"
kubectl -n "${AGENT_NAMESPACE}" get pods -l "app.kubernetes.io/name=${DAEMONSET_NAME}" -o wide
