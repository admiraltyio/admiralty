#!/usr/bin/env bash
set -euo pipefail

source test/e2e/aliases.sh
source test/e2e/mcsa.sh
source test/e2e/webhook_ready.sh

MCSA_URL="$MCSA_RELEASE_URL/install.yaml"

setup() {
  # Install MCSA and bootstrap cluster1 and cluster2 to import from cluster1 (which will host the control plane)
  k1 apply -f "$MCSA_URL"

  ./kubemcsa bootstrap --target-kubeconfig kubeconfig-cluster1 --source-kubeconfig kubeconfig-cluster1
  ./kubemcsa bootstrap --target-kubeconfig kubeconfig-cluster1 --source-kubeconfig kubeconfig-cluster2

  k1 create namespace admiralty
  k2 create namespace admiralty

  k1 label ns admiralty multicluster-service-account=enabled --overwrite

  helm1 install multicluster-scheduler charts/multicluster-scheduler -n admiralty -f test/e2e/with-mcsa/values-cluster1.yaml
  helm2 install multicluster-scheduler charts/multicluster-scheduler -n admiralty -f test/e2e/with-mcsa/values-cluster2.yaml

  cat <<EOF | k1 create -f -
apiVersion: multicluster.admiralty.io/v1alpha1
kind: ServiceAccountImport
metadata:
  name: c1
spec:
  clusterName: cluster1
  namespace: admiralty
  name: multicluster-scheduler-agent-for-scheduler
EOF

  cat <<EOF | k1 create -f -
apiVersion: multicluster.admiralty.io/v1alpha1
kind: ServiceAccountImport
metadata:
  name: c2
spec:
  clusterName: cluster2
  namespace: admiralty
  name: multicluster-scheduler-agent-for-scheduler
EOF

  webhook_ready 1 admiralty multicluster-scheduler-agent multicluster-scheduler-agent multicluster-scheduler-agent-cert
  webhook_ready 2 admiralty multicluster-scheduler-agent multicluster-scheduler-agent multicluster-scheduler-agent-cert
}

tear_down() {
  helm1 delete multicluster-scheduler -n admiralty
  helm2 delete multicluster-scheduler -n admiralty

  k1 delete sai c1 -n admiralty
  k1 delete sai c2 -n admiralty

  k1 label ns admiralty multicluster-service-account-

  k1 delete namespace admiralty
  k2 delete namespace admiralty

  k1 delete -f "$MCSA_URL"
}
