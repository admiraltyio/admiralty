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
source test/e2e/webhook_ready.sh

cert_manager_setup_once() {
  helm repo add jetstack https://charts.jetstack.io
  helm repo update
}

cert_manager_setup() {
  i=$1

  k $i apply --validate=false -f https://github.com/jetstack/cert-manager/releases/download/v0.16.1/cert-manager.crds.yaml
  if ! k $i get ns cert-manager; then
    k $i create ns cert-manager
  fi
  h $i upgrade --install cert-manager jetstack/cert-manager -n cert-manager --version v0.16.1 --wait --debug
  #  webhook_ready $i cert-manager cert-manager-webhook cert-manager-webhook cert-manager-webhook-tls
}

if [[ "${BASH_SOURCE[0]:-}" == "${0}" ]]; then
  cert_manager_setup_one
  cert_manager_setup $1
fi
