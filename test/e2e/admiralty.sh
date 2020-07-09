#!/usr/bin/env bash
set -euo pipefail

source test/e2e/aliases.sh
source test/e2e/kind.sh
source test/e2e/webhook_ready.sh

admiralty_setup() {
  i=$1
  VALUES=$2
  VERSION="$3"

  kind load docker-image quay.io/admiralty/multicluster-scheduler-agent:$VERSION --name cluster$i
  kind load docker-image quay.io/admiralty/multicluster-scheduler-scheduler:$VERSION --name cluster$i
  kind load docker-image quay.io/admiralty/multicluster-scheduler-remove-finalizers:$VERSION --name cluster$i
  kind load docker-image quay.io/admiralty/multicluster-scheduler-restarter:$VERSION --name cluster$i

  if ! k $i get ns admiralty; then
    k $i create namespace admiralty
  fi
  h $i upgrade --install multicluster-scheduler charts/multicluster-scheduler -n admiralty -f $VALUES \
    --set controllerManager.image.tag=$VERSION \
    --set scheduler.image.tag=$VERSION \
    --set postDeleteJob.image.tag=$VERSION \
    --set restarter.image.tag=$VERSION
  k $i delete pod --all -n admiralty

  k $i label ns default multicluster-scheduler=enabled --overwrite
}

if [[ "${BASH_SOURCE[0]:-}" == "${0}" ]]; then
  admiralty_setup "${@}"
fi
