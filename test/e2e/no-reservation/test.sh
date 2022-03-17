#!/usr/bin/env bash
#
# Copyright 2022 The Multicluster-Scheduler Authors.
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
source test/e2e/admiralty.sh

no-reservation_test() {
  # the algorithm without a candidate scheduler marks virtual nodes as unschedulable over multiple scheduling cycles,
  # we need to make sure that it resets those marks after going through all nodes
  # because conditions may have changed when it retries
  k 1 cordon cluster1-control-plane
  k 1 cordon admiralty-default-c2
  k 1 apply -f test/e2e/no-reservation/test.yaml

  export -f no-reservation_test_iteration
  timeout --foreground 60s bash -c "until no-reservation_test_iteration; do sleep 1; done"
  # use --foreground to catch ctrl-c
  # https://unix.stackexchange.com/a/233685

  k 1 uncordon cluster1-control-plane

  k 1 rollout status deploy no-reservation --timeout=30s

  k 1 delete -f test/e2e/no-reservation/test.yaml
  k 1 uncordon admiralty-default-c2
}

no-reservation_test_iteration() {
  set -euo pipefail
  source test/e2e/aliases.sh

  k 1 get pod -l app=no-reservation -o json | jq -e '.items[0].status.conditions[] | select(.type == "PodScheduled") | .reason == "Unschedulable"'
}

if [[ "${BASH_SOURCE[0]:-}" == "${0}" ]]; then
  no-reservation_test "${@}"
fi
