#!/usr/bin/env bash
set -euo pipefail

DIR=$(dirname "$0")

VERSION="$1"

setup_clusters() {
  for CLUSTER in cluster1 cluster2 cluster3; do
    kind create cluster --name $CLUSTER --wait 5m
    kind get kubeconfig --name $CLUSTER --internal >kubeconfig-$CLUSTER
    KUBECONFIG=kubeconfig-$CLUSTER kubectl apply -f "$DIR"/must-run-as-non-root.yaml

    kind load docker-image "quay.io/admiralty/multicluster-scheduler-basic:$VERSION" --name $CLUSTER
    kind load docker-image "quay.io/admiralty/multicluster-scheduler-agent:$VERSION" --name $CLUSTER
    kind load docker-image "quay.io/admiralty/multicluster-scheduler-pod-admission-controller:$VERSION" --name $CLUSTER
  done
}

tear_down_clusters() {
  for CLUSTER in cluster1 cluster2 cluster3; do
    rm -f kubeconfig-$CLUSTER
    kind delete cluster --name $CLUSTER # if exists
  done
}
