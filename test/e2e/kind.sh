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

# images built for kind v0.20.0
# https://github.com/kubernetes-sigs/kind/releases/tag/v0.20.0
declare -A kind_images

kind_images[1.28]="kindest/node:v1.28.0@sha256:b7a4cad12c197af3ba43202d3efe03246b3f0793f162afb40a33c923952d5b31"
kind_images[1.27]="kindest/node:v1.27.3@sha256:3966ac761ae0136263ffdb6cfd4db23ef8a83cba8a463690e98317add2c9ba72"
kind_images[1.26]="kindest/node:v1.26.6@sha256:6e2d8b28a5b601defe327b98bd1c2d1930b49e5d8c512e1895099e4504007adb"
kind_images[1.25]="kindest/node:v1.25.11@sha256:227fa11ce74ea76a0474eeefb84cb75d8dad1b08638371ecf0e86259b35be0c8"
kind_images[1.24]="kindest/node:v1.24.15@sha256:7db4f8bea3e14b82d12e044e25e34bd53754b7f2b0e9d56df21774e6f66a70ab"
kind_images[1.23]="kindest/node:v1.23.17@sha256:59c989ff8a517a93127d4a536e7014d28e235fb3529d9fba91b3951d461edfdb"
kind_images[1.22]="kindest/node:v1.22.17@sha256:f5b2e5698c6c9d6d0adc419c0deae21a425c07d81bbf3b6a6834042f25d4fba2"
kind_images[1.21]="kindest/node:v1.21.14@sha256:8a4e9bb3f415d2bb81629ce33ef9c76ba514c14d707f9797a01e3216376ba093"

K8S_VERSION="${K8S_VERSION:-"1.28"}"

kind_setup() {
  i=$1

  CLUSTER=cluster$i

  if ! kind get clusters | grep $CLUSTER; then
    kind create cluster --name $CLUSTER --wait 5m --image "${kind_images["$K8S_VERSION"]}" --config test/e2e/kind-config.yaml
  fi
  NODE_IP=$(docker inspect "${CLUSTER}-control-plane" --format "{{ .NetworkSettings.Networks.kind.IPAddress }}")
  kind get kubeconfig --name $CLUSTER --internal | \
    sed "s/${CLUSTER}-control-plane/${NODE_IP}/g" >kubeconfig-$CLUSTER
}

if [[ "${BASH_SOURCE[0]:-}" == "${0}" ]]; then
  kind_setup $1
fi
