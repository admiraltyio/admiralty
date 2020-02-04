#!/usr/bin/env bash
set -euo pipefail

source test/e2e/aliases.sh
source test/e2e/webhook_ready.sh

setup() {
  k1 create namespace admiralty
  k2 create namespace admiralty
  helm1 install multicluster-scheduler charts/multicluster-scheduler -n admiralty -f test/e2e/single-namespace/values-cluster1.yaml
  helm2 install multicluster-scheduler charts/multicluster-scheduler -n admiralty -f test/e2e/single-namespace/values-cluster2.yaml

  ./kubemcsa export --kubeconfig kubeconfig-cluster1 multicluster-scheduler-agent-for-scheduler -n admiralty --as c1 | k1 apply -n admiralty -f -
  ./kubemcsa export --kubeconfig kubeconfig-cluster2 multicluster-scheduler-agent-for-scheduler -n admiralty --as c2 | k1 apply -n admiralty -f -

  webhook_ready 1 admiralty multicluster-scheduler-agent multicluster-scheduler-agent multicluster-scheduler-agent-cert
  webhook_ready 2 admiralty multicluster-scheduler-agent multicluster-scheduler-agent multicluster-scheduler-agent-cert
}

tear_down() {
  helm1 delete multicluster-scheduler -n admiralty
  helm2 delete multicluster-scheduler -n admiralty

  k1 delete secret c1 -n admiralty
  k1 delete secret c2 -n admiralty

  k1 delete namespace admiralty
  k2 delete namespace admiralty
}
