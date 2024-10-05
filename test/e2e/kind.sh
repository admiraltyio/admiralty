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

# images built for kind v0.24.0
# https://github.com/kubernetes-sigs/kind/releases/tag/v0.24.0
declare -A kind_images

kind_images[1.31]="kindest/node:v1.31.0@sha256:53df588e04085fd41ae12de0c3fe4c72f7013bba32a20e7325357a1ac94ba865"
kind_images[1.30]="kindest/node:v1.30.4@sha256:976ea815844d5fa93be213437e3ff5754cd599b040946b5cca43ca45c2047114"
kind_images[1.29]="kindest/node:v1.29.8@sha256:d46b7aa29567e93b27f7531d258c372e829d7224b25e3fc6ffdefed12476d3aa"
kind_images[1.28]="kindest/node:v1.28.13@sha256:45d319897776e11167e4698f6b14938eb4d52eb381d9e3d7a9086c16c69a8110"
kind_images[1.27]="kindest/node:v1.27.16@sha256:3fd82731af34efe19cd54ea5c25e882985bafa2c9baefe14f8deab1737d9fabe"


K8S_VERSION="${K8S_VERSION:-"1.29"}"

kind_setup() {
  i=$1

  CLUSTER=cluster$i

  if ! kind get clusters | grep $CLUSTER; then
    kind create cluster --name $CLUSTER --wait 5m --image "${kind_images["$K8S_VERSION"]}"
  fi
  NODE_IP=$(docker inspect "${CLUSTER}-control-plane" --format "{{ .NetworkSettings.Networks.kind.IPAddress }}")
  kind get kubeconfig --name $CLUSTER --internal | \
    sed "s/${CLUSTER}-control-plane/${NODE_IP}/g" >kubeconfig-$CLUSTER
}

if [[ "${BASH_SOURCE[0]:-}" == "${0}" ]]; then
  kind_setup $1
fi
