#!/usr/bin/env bash
set -euo pipefail

source test/e2e/aliases.sh
source test/e2e/webhook_ready.sh

cert_manager_setup_once() {
  helm repo add jetstack https://charts.jetstack.io
  helm repo update
}

cert_manager_setup() {
  i=$1

  k $i apply --validate=false -f https://raw.githubusercontent.com/jetstack/cert-manager/release-0.12/deploy/manifests/00-crds.yaml
  if ! k $i get ns cert-manager; then
    k $i create ns cert-manager
  fi
  h $i upgrade --install cert-manager jetstack/cert-manager -n cert-manager --version v0.12.0 --wait
  #  webhook_ready $i cert-manager cert-manager-webhook cert-manager-webhook cert-manager-webhook-tls
}

if [[ "${BASH_SOURCE[0]:-}" == "${0}" ]]; then
  cert_manager_setup_one
  cert_manager_setup $1
fi
