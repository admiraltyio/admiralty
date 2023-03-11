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

cert_manager_setup_once() {
  helm repo add jetstack https://charts.jetstack.io
  helm repo update
}

cert_manager_setup() {
  i=$1

  h $i upgrade --install cert-manager jetstack/cert-manager \
    --namespace cert-manager --create-namespace \
    --version v1.11.0 --set installCRDs=true \
    --wait --debug --timeout=1m
}

if [[ "${BASH_SOURCE[0]:-}" == "${0}" ]]; then
  cert_manager_setup_one
  cert_manager_setup $1
fi
