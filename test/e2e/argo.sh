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

argo_version=2.8.2
argo_manifest="https://raw.githubusercontent.com/argoproj/argo-workflows/v$argo_version/manifests/install.yaml"
argo_img="argoproj/argoexec:v$argo_version"

argo_setup_once() {
  os=${1:-linux}
  arch=${2:-amd64}

  if
    out=$(./argo version) || true
    echo "$out" | grep "$argo_version"
  then
    echo "argo already downloaded"
  else
    echo "downloading argo"
    curl -Lo argo "https://github.com/argoproj/argo-workflows/releases/download/v$argo_version/argo-$os-$arch"
    chmod +x argo
  fi
}

argo_setup_source() {
  i=$1

  if ! k "$i" get ns argo; then
    k "$i" create ns argo
  fi
  k "$i" apply -n argo -f "$argo_manifest"

  # kind uses containerd not docker so we change the argo executor (default: docker)
  # TODO modify install.yaml instead
  k "$i" patch cm -n argo workflow-controller-configmap --patch '{"data":{"config":"{\"containerRuntimeExecutor\":\"k8sapi\"}"}}'
  k "$i" delete pod --all -n argo # reload config map

  k "$i" apply -f examples/argo-workflows/_service-account.yaml
}

argo_setup_target() {
  i=$1

  k "$i" apply -f examples/argo-workflows/_service-account.yaml
}

argo_test() {
  i=$1
  j=$2

  KUBECONFIG=kubeconfig-cluster$i ./argo submit --serviceaccount argo-workflow --wait examples/argo-workflows/blog-scenario-a-multicluster.yaml
  [ $(k "$j" get pod -l multicluster.admiralty.io/workflow | wc -l) -gt 1 ]
  KUBECONFIG=kubeconfig-cluster$i ./argo delete --all
}

if [[ "${BASH_SOURCE[0]:-}" == "${0}" ]]; then
  argo_test "${@}"
fi
