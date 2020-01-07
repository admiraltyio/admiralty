#!/usr/bin/env bash
set -euo pipefail

source test/e2e/aliases.sh
source test/e2e/webhook_ready.sh

setup_cert_manager() {
  helm repo add jetstack https://charts.jetstack.io
  helm repo update

  k1 apply --validate=false -f https://raw.githubusercontent.com/jetstack/cert-manager/release-0.12/deploy/manifests/00-crds.yaml
  helm1 install cert-manager jetstack/cert-manager

  k2 apply --validate=false -f https://raw.githubusercontent.com/jetstack/cert-manager/release-0.12/deploy/manifests/00-crds.yaml
  helm2 install cert-manager jetstack/cert-manager

  webhook_ready 1 default cert-manager-webhook cert-manager-webhook cert-manager-webhook-tls
  webhook_ready 2 default cert-manager-webhook cert-manager-webhook cert-manager-webhook-tls
}

tear_down_cert_manager() {
  helm1 delete cert-manager
  helm2 delete cert-manager
}
