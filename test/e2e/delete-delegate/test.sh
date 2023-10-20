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

delete-delegate_test() {
  i=$1
  j=$2

  k $j label node --all a=b --overwrite
  k $i apply -f test/e2e/delete-delegate/test.yaml
  k $i wait pod test-delete-delegate --for=condition=PodScheduled

  # when the cluster connection is interrupted for more than a minute,
  # the delegate pod (with restart policy always) should be recreated

  k $i scale deploy -n admiralty admiralty-controller-manager --replicas=0
  uid="$(k $j get pod -l multicluster.admiralty.io/app=delete-delegate -o json | jq -er '.items[0].metadata.uid')"
  echo $uid
  k $j delete pod -l multicluster.admiralty.io/app=delete-delegate

  export -f delete-delegate_test_iteration
  timeout --foreground 120s bash -c "until delete-delegate_test_iteration $j $uid; do sleep 1; done"
  # use --foreground to catch ctrl-c
  # https://unix.stackexchange.com/a/233685
  k $j wait pod -l multicluster.admiralty.io/app=delete-delegate --for=condition=PodScheduled

  # when the cluster connection is working, the proxy pod should be deleted
  # to respect the invariant that pods can't resuscitate

  k $i scale deploy -n admiralty admiralty-controller-manager --replicas=2
  k $j delete pod -l multicluster.admiralty.io/app=delete-delegate --wait --timeout=30s

  k $i wait pod test-delete-delegate --for=delete

  k $j label node --all a-
}

delete-delegate_test_iteration() {
  j=$1
  old_uid=$2

  set -euo pipefail
  source test/e2e/aliases.sh

  new_uid="$(k "$j" get pod -l multicluster.admiralty.io/app=delete-delegate -o json | jq -er '.items[0].metadata.uid')" || return 1
  [ "$new_uid" != "$old_uid" ] || return 1
}

if [[ "${BASH_SOURCE[0]:-}" == "${0}" ]]; then
  delete-delegate_test "${@}"
fi
