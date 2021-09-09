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

# images built for kind v0.11.1
# https://github.com/kubernetes-sigs/kind/releases/tag/v0.11.1
declare -A kind_images
kind_images[1.21]="kindest/node:v1.21.1@sha256:69860bda5563ac81e3c0057d654b5253219618a22ec3a346306239bba8cfa1a6"
kind_images[1.20]="kindest/node:v1.20.7@sha256:cbeaf907fc78ac97ce7b625e4bf0de16e3ea725daf6b04f930bd14c67c671ff9"
kind_images[1.19]="kindest/node:v1.19.11@sha256:07db187ae84b4b7de440a73886f008cf903fcf5764ba8106a9fd5243d6f32729"
kind_images[1.18]="kindest/node:v1.18.19@sha256:7af1492e19b3192a79f606e43c35fb741e520d195f96399284515f077b3b622c"
kind_images[1.17]="kindest/node:v1.17.17@sha256:66f1d0d91a88b8a001811e2f1054af60eef3b669a9a74f9b6db871f2f1eeed00"
kind_images[1.16]="kindest/node:v1.16.15@sha256:83067ed51bf2a3395b24687094e283a7c7c865ccc12a8b1d7aa673ba0c5e8861"
kind_images[1.15]="kindest/node:v1.15.12@sha256:b920920e1eda689d9936dfcf7332701e80be12566999152626b2c9d730397a95"
kind_images[1.14]="kindest/node:v1.14.10@sha256:f8a66ef82822ab4f7569e91a5bccaf27bceee135c1457c512e54de8c6f7219f8"
# "known to work well" (k8s 1.22 wasn't released when kind 0.11.1 was released)
kind_images[1.22]="kindest/node:v1.22.0@sha256:b8bda84bb3a190e6e028b1760d277454a72267a5454b57db34437c34a588d047"

K8S_VERSION="${K8S_VERSION:-"1.21"}"

kind_setup() {
  i=$1

  CLUSTER=cluster$i

  if ! kind get clusters | grep $CLUSTER; then
    kind create cluster --name $CLUSTER --wait 5m --image "${kind_images["$K8S_VERSION"]}"
  fi
  NODE_IP=$(docker inspect "${CLUSTER}-control-plane" --format "{{ .NetworkSettings.Networks.kind.IPAddress }}")
  kind get kubeconfig --name $CLUSTER --internal | \
    sed "s/${CLUSTER}-control-plane/${NODE_IP}/g" >kubeconfig-$CLUSTER
  k $i apply -f test/e2e/must-run-as-non-root.yaml
}

if [[ "${BASH_SOURCE[0]:-}" == "${0}" ]]; then
  kind_setup $1
fi
