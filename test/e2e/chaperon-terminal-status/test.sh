#!/usr/bin/env bash
#
# Copyright The Multicluster-Scheduler Authors.
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

chaperon-terminal-status_test() {
  i=$1
  j=$2
  k $j label node --all a=b --overwrite

  k $i create serviceaccount test-chaperon-terminal-status
  k $i apply -f test/e2e/chaperon-terminal-status/test.yaml
  while [ $(k $i get pod test-chaperon-terminal-status | wc -l) = 0 ]; do sleep 1; done
  uid="$(k $i get pod test-chaperon-terminal-status -o json | jq -er '.metadata.uid')"

  while [ $(k $j get podchaperon -l multicluster.admiralty.io/parent-uid="$uid" | wc -l) = 0 ]; do sleep 1; done
  pc="$(k $j get podchaperon -l multicluster.admiralty.io/parent-uid="$uid" -o json)"
  pcName="$(echo $pc | jq -er '.items[0].metadata.name')"
  k $j wait podchaperon $pcName --for='jsonpath={.status.conditions[?(@.type=="PodScheduled")].status}=False'

  k $j create serviceaccount test-chaperon-terminal-status
  k $j wait podchaperon $pcName --for='jsonpath={.status.conditions[?(@.type=="PodScheduled")].status}=True'
  k $i wait pod test-chaperon-terminal-status --for=condition=ContainersReady

  k $i delete -f test/e2e/chaperon-terminal-status/test.yaml
  k $j label node --all a-
}

if [[ "${BASH_SOURCE[0]:-}" == "${0}" ]]; then
  chaperon-terminal-status_test "${@}"
fi
