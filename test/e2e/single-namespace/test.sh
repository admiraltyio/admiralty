#!/usr/bin/env bash
set -euo pipefail

source test/e2e/aliases.sh

setup() {
  helm1 upgrade --install multicluster-scheduler charts/multicluster-scheduler -f test/e2e/single-namespace/values-cluster1.yaml
  helm2 upgrade --install multicluster-scheduler charts/multicluster-scheduler -f test/e2e/single-namespace/values-cluster2.yaml

  ./kubemcsa export --kubeconfig kubeconfig-cluster1 c1 --as remote | k1 apply -f -
  ./kubemcsa export --kubeconfig kubeconfig-cluster1 c2 --as remote | k2 apply -f -
}

tear_down() {
  helm1 delete multicluster-scheduler
  helm2 delete multicluster-scheduler

  k1 delete secret remote
  k2 delete secret remote
}
