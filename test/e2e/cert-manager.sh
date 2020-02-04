#!/usr/bin/env bash
set -euo pipefail

source test/e2e/aliases.sh
source test/e2e/webhook_ready.sh

setup_cert_manager() {
  helm repo add jetstack https://charts.jetstack.io
  helm repo update

  k1 apply --validate=false -f https://raw.githubusercontent.com/jetstack/cert-manager/release-0.12/deploy/manifests/00-crds.yaml
  k1 create ns cert-manager
  helm1 install cert-manager jetstack/cert-manager -n cert-manager --version v0.12.0 --wait

  k2 apply --validate=false -f https://raw.githubusercontent.com/jetstack/cert-manager/release-0.12/deploy/manifests/00-crds.yaml
  k2 create ns cert-manager
  helm2 install cert-manager jetstack/cert-manager -n cert-manager --version v0.12.0 --wait

  webhook_ready 1 cert-manager cert-manager-webhook cert-manager-webhook cert-manager-webhook-tls
  webhook_ready 2 cert-manager cert-manager-webhook cert-manager-webhook cert-manager-webhook-tls
}

tear_down_cert_manager() {
  helm1 delete cert-manager -n cert-manager
  helm2 delete cert-manager -n cert-manager
  k1 delete ns cert-manager
  k2 delete ns cert-manager
}
