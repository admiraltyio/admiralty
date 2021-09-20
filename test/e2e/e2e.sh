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
source test/e2e/k8s/kind.sh
source test/e2e/k8s/eks.sh
source test/e2e/follow/test.sh
source test/e2e/logs/test.sh
source test/e2e/exec/test.sh
source test/e2e/ingress/test.sh
source test/e2e/virtual-node-labels/test.sh
source test/e2e/webhook_ready.sh
source test/e2e/no-rogue-finalizer/test.sh

K8S_DISTRIB="${K8S_DISTRIB:-kind}"

argo_setup_once
cert_manager_setup_once

case "$K8S_DISTRIB" in
  kind)
    kind_setup_once
    create_cluster=kind_setup
    async=false
    ;;
  eks)
    eks_setup_once
    create_cluster=eks_setup
    async=true
    ;;
  *)
    echo "unknown Kubernetes distribution $K8S_DISTRIB" >&2
    exit 1
    ;;
esac

cluster_dump() {
  if [ $? -ne 0 ]; then
    for i in 1 2; do
      k $i cluster-info dump -A --output-directory cluster-dump/$i
    done
  fi
}
trap cluster_dump EXIT

pids=()

for i in 1 2; do
  if [ $async = true ]; then
    $create_cluster $i &
    pids+=($!)
  else
    $create_cluster $i
  fi
done

for pid in "${pids[@]}"; do
  wait $pid
done

for i in 1 2; do
  cert_manager_setup $i
  REGISTRY=$registry admiralty_setup $i test/e2e/values.yaml
done

for j in 1 2 3; do
  admiralty_connect 1 $j
done

argo_setup_source 1
argo_setup_target 2
webhook_ready 1 admiralty multicluster-scheduler-controller-manager multicluster-scheduler multicluster-scheduler-cert

# TODO simulate route controller not being able to create network routes to virtual nodes
#k 1 taint nodes -l virtual-kubelet.io/provider=admiralty node.kubernetes.io/network-unavailable=:NoSchedule
# unfortunately, we can't use kubectl to taint nodes with node.kubernetes.io/network-unavailable
# some system defaulting admission controller overwriting the reserved taint?

argo_test 1 2
follow_test 1 2
#if [[ "$K8S_DISTRIB" != eks || "$K8S_VERSION" == "1.17" || "$K8S_VERSION" == "1.18" ]]; then
logs_test 1 2
exec_test 1 2
#fi
ingress_test 1 2
virtual-node-labels_test 1 2
no-rogue-finalizer_test

echo "ALL SUCCEEDED"
