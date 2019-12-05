#!/usr/bin/env bash
set -euo pipefail

source test/e2e/aliases.sh
source test/e2e/mcsa.sh

MCSA_URL="$MCSA_RELEASE_URL/install.yaml"

setup() {
  # Install MCSA and bootstrap cluster1 and cluster2 to import from cluster1 (which will host the control plane)
  k1 apply -f "$MCSA_URL"
  k2 apply -f "$MCSA_URL"

  ./kubemcsa bootstrap --target-kubeconfig kubeconfig-cluster1 --source-kubeconfig kubeconfig-cluster1
  ./kubemcsa bootstrap --target-kubeconfig kubeconfig-cluster2 --source-kubeconfig kubeconfig-cluster1

  k1 label ns default multicluster-service-account=enabled --overwrite
  k2 label ns default multicluster-service-account=enabled --overwrite

  helm1 upgrade --install multicluster-scheduler charts/multicluster-scheduler -f test/e2e/with-mcsa/values-cluster1.yaml
  helm2 upgrade --install multicluster-scheduler charts/multicluster-scheduler -f test/e2e/with-mcsa/values-cluster2.yaml

  cat <<EOF | k1 create -f -
apiVersion: multicluster.admiralty.io/v1alpha1
kind: ServiceAccountImport
metadata:
  name: remote
spec:
  clusterName: cluster1
  namespace: default
  name: c1
EOF

  cat <<EOF | k2 create -f -
apiVersion: multicluster.admiralty.io/v1alpha1
kind: ServiceAccountImport
metadata:
  name: remote
spec:
  clusterName: cluster1
  namespace: default
  name: c2
EOF
}

tear_down() {
  helm1 delete multicluster-scheduler
  helm2 delete multicluster-scheduler

  k1 delete sai remote
  k2 delete sai remote

  k1 label ns default multicluster-service-account-
  k2 label ns default multicluster-service-account-

  k2 delete -f "$MCSA_URL"
  k1 delete -f "$MCSA_URL"
}
