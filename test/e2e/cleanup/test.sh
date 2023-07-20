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
source test/e2e/admiralty.sh

cleanup_test() {
  i=$1

  k $i apply -f test/e2e/cleanup/test.yaml
  k $i rollout status deploy cleanup
  target="$(k $i get pod -l app=cleanup -o json | jq -er '.items[0].metadata.finalizers[0] | split("-") | .[1]')"
  k $i delete target $target

  export -f cleanup_test_iteration
  timeout --foreground 180s bash -c "until cleanup_test_iteration $i; do sleep 1; done"
  # use --foreground to catch ctrl-c
  # https://unix.stackexchange.com/a/233685

  admiralty_connect $i "${target: -1}"
  k $i wait --for condition=available --timeout=120s deployment multicluster-scheduler-controller-manager -n admiralty
  k $i delete -f test/e2e/cleanup/test.yaml
}

cleanup_test_iteration() {
  i=$1

  set -euo pipefail
  source test/e2e/aliases.sh

  [ $(k $i get pod -l app=cleanup -o json | jq -e '.items[0].metadata.finalizers | length') -eq 0 ] || return 1
}

if [[ "${BASH_SOURCE[0]:-}" == "${0}" ]]; then
  cleanup_test "${@}"
fi
