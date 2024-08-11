#!/usr/bin/env bash
#
# Copyright 2024 The Multicluster-Scheduler Authors.
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

plugin-config-scoring-strategy_test() {
  i=$1
  j=$2
  k $j label node --all a=b --overwrite

  # Find the node with the highest CPU utilization
  nodes=$(k $j get nodes -o name | sed s/"node\/"//)
  most_allocated=""
  most_cpu_requests="0"
  for node in $nodes; do
      cpu_requests=$(k $j describe node "$node" | grep -A5 "Allocated" | awk '{print $3}' | sed -n '5p' | tr -d '()%')
      echo "node: $node, cpu requests: $cpu_requests"
      if [[ "$cpu_requests" -ge "$most_cpu_requests" ]]
      then
        most_cpu_requests=$cpu_requests
        most_allocated=$node
      fi
  done

  k $i apply -f test/e2e/plugin-config-scoring-strategy/test.yaml
  k $i wait pod test-scoring-strategy --for=condition=PodScheduled

  node_scheduled="$(k $j get pod -o json | jq -er '.items[0].spec.nodeName')"
  k $i delete -f test/e2e/plugin-config-scoring-strategy/test.yaml
  k $j label node --all a-
  if [ "$node_scheduled" != "$most_allocated" ]
  then
      echo "delegate pod was assigned to an unexpected node. Expected node $most_allocated, got assigned to $node_scheduled"
      exit 1
  fi
}

if [[ "${BASH_SOURCE[0]:-}" == "${0}" ]]; then
  plugin-config-scoring-strategy_test "${@}"
fi
