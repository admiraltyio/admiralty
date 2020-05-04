#!/usr/bin/env bash
set -euo pipefail

klum_setup() {
  i=$1

  k $i apply -f https://raw.githubusercontent.com/ibuildthecloud/klum/v0.0.1/deploy.yaml
  while ! k $i wait crd users.klum.cattle.io --for condition=established; do
    sleep 1
  done
}
