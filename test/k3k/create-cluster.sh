#!/usr/bin/env bash
# Create a k3k cluster, wait for it to be Ready, and write its kubeconfig.
# Prints the generated cluster name to stdout on success.
#
# Required env:
#   NAMESPACE       Kubernetes namespace to create the cluster in
#   KUBECONFIG_OUT  Path to write the resulting kubeconfig
#
# Optional env (set under Tekton to enable ownerReference-based GC + labels):
#   PIPELINE_NAME       Tekton Pipeline name
#   PIPELINERUN_NAME    Tekton PipelineRun name
#   PIPELINERUN_UID     Tekton PipelineRun UID

set -euo pipefail

: "${NAMESPACE:?NAMESPACE must be set}"
: "${KUBECONFIG_OUT:?KUBECONFIG_OUT must be set}"

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)

cluster_name=$(kubectl create -n "${NAMESPACE}" -f "${script_dir}/test-cluster.yaml" -ojson | jq -r '.metadata.name')

if [[ -n "${PIPELINERUN_NAME:-}" && -n "${PIPELINERUN_UID:-}" ]]; then
  owner_ref=$(jo \
    apiVersion=tekton.dev/v1 \
    kind=PipelineRun \
    name="${PIPELINERUN_NAME}" \
    uid="${PIPELINERUN_UID}" \
    blockOwnerDeletion=false)
  patch=$(jo metadata=$(jo "ownerReferences[]=${owner_ref}"))

  kubectl patch -n "${NAMESPACE}" "clusters.k3k.io/${cluster_name}" --type=merge -p "${patch}" >&2

  kubectl label -n "${NAMESPACE}" "clusters.k3k.io/${cluster_name}" \
    "tekton.dev/memberOf=tasks" \
    "tekton.dev/pipeline=${PIPELINE_NAME:-}" \
    "tekton.dev/pipelineRun=${PIPELINERUN_NAME}" \
    "tekton.dev/pipelineRunUID=${PIPELINERUN_UID}" >&2
fi

kubectl wait --for=condition=Ready "clusters.k3k.io/${cluster_name}" -n "${NAMESPACE}" --timeout 5m >&2
kubectl wait --for=create "secret/k3k-${cluster_name}-kubeconfig" -n "${NAMESPACE}" --timeout 5m >&2

mkdir -p "$(dirname "${KUBECONFIG_OUT}")"
kubectl get -ojson -n "${NAMESPACE}" "secret/k3k-${cluster_name}-kubeconfig" \
  | jq -r '.data["kubeconfig.yaml"]' \
  | base64 -d > "${KUBECONFIG_OUT}"

echo "${cluster_name}"
