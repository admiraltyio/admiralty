#!/usr/bin/env bash
set -euo pipefail

source test/e2e/aliases.sh

setup() {
  k1 create ns c1
  k1 create ns c2

  k1 create ns multicluster-scheduler
  k2 create ns multicluster-scheduler

  helm1 upgrade --install multicluster-scheduler charts/multicluster-scheduler -n multicluster-scheduler -f test/e2e/cluster-namespaces/values-cluster1.yaml
  helm2 upgrade --install multicluster-scheduler charts/multicluster-scheduler -n multicluster-scheduler -f test/e2e/cluster-namespaces/values-cluster2.yaml

  ./kubemcsa export --kubeconfig kubeconfig-cluster1 member -n c1 --as remote | k1 apply -n multicluster-scheduler -f -
  ./kubemcsa export --kubeconfig kubeconfig-cluster1 member -n c2 --as remote | k2 apply -n multicluster-scheduler -f -
}

tear_down() {
  helm1 delete multicluster-scheduler
  helm2 delete multicluster-scheduler

  k1 delete secret remote -n multicluster-scheduler
  k2 delete secret remote -n multicluster-scheduler

  k1 delete ns c1
  k1 delete ns c2

  k1 delete ns multicluster-scheduler
  k2 delete ns multicluster-scheduler
}
