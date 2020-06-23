#!/usr/bin/env bash
set -euo pipefail

VERSION="$1"

source test/e2e/aliases.sh
source test/e2e/admiralty.sh
source test/e2e/argo.sh
source test/e2e/cert-manager.sh
source test/e2e/kind.sh
source test/e2e/klum.sh
source test/e2e/mcsa.sh
source test/e2e/follow/test.sh

argo_setup_once
cert_manager_setup_once
mcsa_setup_once

for i in 1 2; do
  kind_setup $i
  cert_manager_setup $i
  admiralty_setup $i test/e2e/argo-workflow/values-cluster$i.yaml $VERSION
done

klum_setup 2
k 2 apply -f test/e2e/argo-workflow/cluster1-on-cluster2.yaml
./kubemcsa export --kubeconfig kubeconfig-cluster2 cluster1 -n klum --as c2 | k 1 apply -n admiralty -f -

argo_setup_source 1
argo_setup_target 2
#webhook_ready 1 admiralty multicluster-scheduler-controller-manager multicluster-scheduler multicluster-scheduler-cert
argo_test 1 2

follow_test 1 2

# check that we didn't add finalizers to uncontrolled resources
finalizer="multicluster.admiralty.io/multiclusterForegroundDeletion"
[ "$(k 1 get pod -A -o custom-columns=FINALIZERS:.metadata.finalizers | grep -c "$finalizer")" -eq 0 ]

echo "ALL SUCCEEDED"
