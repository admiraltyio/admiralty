#!/usr/bin/env bash
set -euo pipefail

MCSA_RELEASE_URL=https://github.com/admiraltyio/multicluster-service-account/releases/download/v0.6.1

install_kubemcsa() {
  OS=linux
  ARCH=amd64

  curl -Lo kubemcsa "$MCSA_RELEASE_URL/kubemcsa-$OS-$ARCH"
  chmod +x kubemcsa
}
