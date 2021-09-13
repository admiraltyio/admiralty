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

virtual-node-labels_test() {
  i=$1
  j=$2

  k $j label node --all kubernetes.azure.com/cluster=cluster$j --overwrite
  while [[ "$(k $i get node admiralty-default-c$j -o json | jq -r '.metadata.labels["kubernetes.azure.com/cluster"]')" != cluster$j ]]; do sleep 1; done
  k $i patch target c$j --type=merge -p '{"spec":{"excludedLabelsRegexp": "^kubernetes\\.azure\\.com/cluster="}}'
  while [[ "$(k $i get node admiralty-default-c$j -o json | jq -r '.metadata.labels["kubernetes.azure.com/cluster"]')" == cluster$j ]]; do sleep 1; done
  k $i patch target c$j --type=merge -p '{"spec":{"excludedLabelsRegexp": null}}'
  k $j label node --all kubernetes.azure.com/cluster-
}

if [[ "${BASH_SOURCE[0]:-}" == "${0}" ]]; then
  virtual-node-labels_test "${@}"
fi
