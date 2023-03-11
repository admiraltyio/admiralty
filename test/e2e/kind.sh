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

# images built for kind v0.11.1
# https://github.com/kubernetes-sigs/kind/releases/tag/v0.11.1
declare -A kind_images

kind_images[1.25]="kindest/node:v1.25.3@sha256:f52781bc0d7a19fb6c405c2af83abfeb311f130707a0e219175677e366cc45d1"
kind_images[1.24]="kindest/node:v1.24.7@sha256:577c630ce8e509131eab1aea12c022190978dd2f745aac5eb1fe65c0807eb315"
kind_images[1.23]="kindest/node:v1.23.13@sha256:ef453bb7c79f0e3caba88d2067d4196f427794086a7d0df8df4f019d5e336b61"
kind_images[1.22]="kindest/node:v1.22.15@sha256:7d9708c4b0873f0fe2e171e2b1b7f45ae89482617778c1c875f1053d4cef2e41"
kind_images[1.21]="kindest/node:v1.21.14@sha256:9d9eb5fb26b4fbc0c6d95fa8c790414f9750dd583f5d7cee45d92e8c26670aa1"
kind_images[1.20]="kindest/node:v1.20.15@sha256:a32bf55309294120616886b5338f95dd98a2f7231519c7dedcec32ba29699394"
kind_images[1.19]="kindest/node:v1.19.16@sha256:476cb3269232888437b61deca013832fee41f9f074f9bed79f57e4280f7c48b7"
# "known to work well" (k8s 1.26 wasn't released when kind 0.17.0 was released)
kind_images[1.26]="kindest/node:v1.26.0@sha256:691e24bd2417609db7e589e1a479b902d2e209892a10ce375fab60a8407c7352"

K8S_VERSION="${K8S_VERSION:-"1.26"}"

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
