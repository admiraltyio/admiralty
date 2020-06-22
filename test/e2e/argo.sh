#!/usr/bin/env bash
set -euo pipefail

source test/e2e/aliases.sh
source test/e2e/kind.sh

ARGO_VERSION=2.8.2
ARGO_MANIFEST="https://raw.githubusercontent.com/argoproj/argo/v$ARGO_VERSION/manifests/install.yaml"
ARGO_IMG="argoproj/argoexec:v$ARGO_VERSION"

argo_setup_once() {
  OS=${1:-linux}
  ARCH=${2:-amd64}

  if ./argo version | grep "$ARGO_VERSION"; then
    echo "argo already downloaded"
  else
    echo "downloading argo"
    curl -Lo argo "https://github.com/argoproj/argo/releases/download/v$ARGO_VERSION/argo-$OS-$ARCH"
    chmod +x argo
  fi

  # to speed up container creations (loaded by kind in argo_setup_source and argo_setup_target)
  docker pull "$ARGO_IMG" # may already be on host
}

argo_setup_source() {
  i=$1

  if ! k $i get ns argo; then
    k $i create ns argo
  fi
  k $i apply -n argo -f "$ARGO_MANIFEST"

  # kind uses containerd not docker so we change the argo executor (default: docker)
  # TODO modify install.yaml instead
  k $i patch cm -n argo workflow-controller-configmap --patch '{"data":{"config":"{\"containerRuntimeExecutor\":\"k8sapi\"}"}}'
  k $i delete pod --all -n argo # reload config map

  k $i apply -f examples/argo-workflows/_service-account.yaml

  # speed up container creations
  kind load docker-image "$ARGO_IMG" --name cluster$i
}

argo_setup_target() {
  i=$1

  k $i apply -f examples/argo-workflows/_service-account.yaml

  # speed up container creations
  kind load docker-image "$ARGO_IMG" --name cluster$i
}

argo_test() {
  i=$1
  j=$2

  KUBECONFIG=kubeconfig-cluster$i ./argo submit --serviceaccount argo-workflow --wait examples/argo-workflows/blog-scenario-a-multicluster.yaml

  if [ $(k $j get pod -l multicluster.admiralty.io/workflow | wc -l) -gt 1 ]; then
    echo "SUCCESS"
  else
    echo "FAILURE"
    exit 1
  fi

  KUBECONFIG=kubeconfig-cluster$i ./argo delete --all
}

if [[ "${BASH_SOURCE[0]:-}" == "${0}" ]]; then
  argo_test "${@}"
fi
