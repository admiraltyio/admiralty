#!/usr/bin/env bash
#
# Copyright 2021 The Multicluster-Scheduler Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

set -euo pipefail

source test/e2e/aliases.sh
source test/e2e/admiralty.sh
source test/e2e/argo.sh
source test/e2e/cert-manager.sh
source test/e2e/kind.sh
source test/e2e/follow/test.sh
source test/e2e/logs/test.sh
source test/e2e/exec/test.sh
source test/e2e/ingress/test.sh
source test/e2e/webhook_ready.sh
source test/e2e/no-rogue-finalizer/test.sh

argo_setup_once
cert_manager_setup_once

cluster_dump() {
  if [ $? -ne 0 ]; then
    k 1 cluster-info dump -A --output-directory cluster-dump/1
    k 2 cluster-info dump -A --output-directory cluster-dump/2
  fi
}
trap cluster_dump EXIT

for i in 1 2; do
  kind_setup $i
  cert_manager_setup $i
  admiralty_setup $i test/e2e/values.yaml
done

k 2 apply -f test/e2e/topologies/namespaced-burst/cluster2/source.yaml
while ! k 2 get sa cluster1; do sleep 1; done

SECRET_NAME=$(k 2 get serviceaccount cluster1 -o json | jq -r .secrets[0].name)
TOKEN=$(k 2 get secret $SECRET_NAME -o json | jq -r .data.token | base64 --decode)
KUBECONFIG=$(k 2 config view --minify --raw -o json | jq '.users[0].user={token:"'$TOKEN'"} | .contexts[0].context.namespace="default"')
k 1 create secret generic c2 --from-literal=config="$KUBECONFIG" --dry-run -o yaml | k 1 apply -f -

k 1 apply -f test/e2e/topologies/namespaced-burst/cluster1/targets.yaml

argo_setup_source 1
argo_setup_target 2
webhook_ready 1 admiralty multicluster-scheduler-controller-manager multicluster-scheduler multicluster-scheduler-cert

# TODO simulate route controller not being able to create network routes to virtual nodes
#k 1 taint nodes -l virtual-kubelet.io/provider=admiralty node.kubernetes.io/network-unavailable=:NoSchedule
# unfortunately, we can't use kubectl to taint nodes with node.kubernetes.io/network-unavailable
# some system defaulting admission controller overwriting the reserved taint?

argo_test 1 2
follow_test 1 2
logs_test 1 2
exec_test 1 2
ingress_test 1 2
no-rogue-finalizer_test

echo "ALL SUCCEEDED"
