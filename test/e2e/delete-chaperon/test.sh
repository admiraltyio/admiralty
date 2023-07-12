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

delete-chaperon_test() {
  i=$1

  k $i apply -f test/e2e/delete-chaperon/test.yaml
  k $i wait pod test-delete-chaperon --for=condition=PodScheduled
  target="$(k $i get pod test-delete-chaperon -o json | jq -er '.spec.nodeName')"
  j="${target: -1}"
  uid="$(k $i get pod test-delete-chaperon -o json | jq -er '.metadata.uid')"
  k $j delete podchaperon -l multicluster.admiralty.io/parent-uid="$uid" --wait --timeout=30s
  k $i wait pod test-delete-chaperon --for=delete
}

if [[ "${BASH_SOURCE[0]:-}" == "${0}" ]]; then
  delete-chaperon_test "${@}"
fi
