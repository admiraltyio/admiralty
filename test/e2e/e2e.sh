#!/usr/bin/env bash
#
# Copyright 2023 The Multicluster-Scheduler Authors.
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
source test/e2e/virtual-node-labels/test.sh
source test/e2e/cleanup/test.sh
source test/e2e/no-reservation/test.sh
source test/e2e/webhook_ready.sh
source test/e2e/no-rogue-finalizer/test.sh
source test/e2e/delete-chaperon/test.sh
source test/e2e/delete-delegate/test.sh
source test/e2e/plugin-config-scoring-strategy/test.sh

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

for j in 1 2 3; do
  admiralty_connect 1 $j
done

# fix GH issue #119: self target in other namespace
admiralty_connect 1 1 other

argo_setup_source 1
argo_setup_target 2
webhook_ready 1 admiralty admiralty-controller-manager admiralty admiralty-cert

# TODO simulate route controller not being able to create network routes to virtual nodes
#k 1 taint nodes -l virtual-kubelet.io/provider=admiralty node.kubernetes.io/network-unavailable=:NoSchedule
# unfortunately, we can't use kubectl to taint nodes with node.kubernetes.io/network-unavailable
# some system defaulting admission controller overwriting the reserved taint?

# fix GH issue #152: different default priority in target cluster
k 2 apply -f test/e2e/priorityclass.yaml

argo_test 1 2
follow_test 1 2
logs_test 1 2
exec_test 1 2
ingress_test 1 2
virtual-node-labels_test 1 2
cleanup_test 1
delete-chaperon_test 1
delete-delegate_test 1 2
plugin-config-scoring-strategy_test 1 2
no-reservation_test

no-rogue-finalizer_test

echo "ALL SUCCEEDED"
