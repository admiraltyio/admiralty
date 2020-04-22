#!/usr/bin/env bash
set -euo pipefail

source test/e2e/webhook_ready.sh

MCSA_VERSION=0.6.1
MCSA_RELEASE_URL="https://github.com/admiraltyio/multicluster-service-account/releases/download/v$MCSA_VERSION"

mcsa_setup_once() {
  OS=${1:-linux}
  ARCH=${2:-amd64}

  if v="$(./kubemcsa --version 2>&1)" && [[ "$v" == "$MCSA_VERSION" ]]; then
    echo "kubemcsa $MCSA_VERSION already downloaded"
    return 0
  fi

  echo "downloading kubemcsa $MCSA_VERSION"

  curl -Lo kubemcsa "$MCSA_RELEASE_URL/kubemcsa-$OS-$ARCH"
  chmod +x kubemcsa
}

mcsa_setup() {
  i=$1

  k $i apply -f "$MCSA_RELEASE_URL/install.yaml"
  #  webhook_ready $i admiralty multicluster-scheduler-agent multicluster-scheduler-agent multicluster-scheduler-agent-cert
  k $i label ns admiralty multicluster-service-account=enabled --overwrite
}

mcsa_bootstrap() {
  i=$1
  j=$2

  ./kubemcsa bootstrap --target-kubeconfig kubeconfig-cluster$i --source-kubeconfig kubeconfig-cluster$j
}

if [[ "${BASH_SOURCE[0]:-}" == "${0}" ]]; then
  mcsa_setup_once "${@}"
fi
