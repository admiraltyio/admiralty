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
source test/e2e/webhook_ready.sh

argo_setup_once
cert_manager_setup_once
#mcsa_setup_once

for i in 1 2; do
  kind_setup $i
  cert_manager_setup $i
  admiralty_setup $i test/e2e/argo-workflow/values-cluster$i.yaml $VERSION
done

klum_setup 2
k 2 apply -f test/e2e/argo-workflow/cluster1-on-cluster2.yaml
while ! k 2 get sa cluster1 -n klum; do sleep 1; done
#./kubemcsa export --kubeconfig kubeconfig-cluster2 cluster1 -n klum --as c2 | k 1 apply -n admiralty -f -

SECRET_NAME=$(k 2 get serviceaccount cluster1 -n klum -o json | jq -r .secrets[0].name)
TOKEN=$(k 2 get secret $SECRET_NAME -n klum -o json | jq -r .data.token | base64 --decode)
KUBECONFIG=$(k 2 config view --minify --raw -o json | jq '.users[0].user={token:"'$TOKEN'"} | .contexts[0].context.namespace="default"')
k 1 create secret generic c2 -n admiralty --from-literal=config="$KUBECONFIG" --dry-run -o yaml | k 1 apply -f -

argo_setup_source 1
argo_setup_target 2
webhook_ready 1 admiralty multicluster-scheduler-controller-manager multicluster-scheduler multicluster-scheduler-cert

cluster_dump() {
  if [ $? -ne 0 ]; then
    k 1 cluster-info dump -A --output-directory cluster-dump/1
    k 2 cluster-info dump -A --output-directory cluster-dump/2
  fi
}
trap cluster_dump EXIT

argo_test 1 2
follow_test 1 2

# check that we didn't add finalizers to uncontrolled resources
finalizer="multicluster.admiralty.io/multiclusterForegroundDeletion"
for resource in pods configmaps secrets services; do
  [ $(k 1 get $resource -A -o custom-columns=FINALIZERS:.metadata.finalizers | grep -c $finalizer) -eq 0 ]
done

echo "ALL SUCCEEDED"
