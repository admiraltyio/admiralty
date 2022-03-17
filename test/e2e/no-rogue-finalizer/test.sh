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

no-rogue-finalizer_test() {
  # give the controllers 30s to clean up after other test objects have been deleted
  export -f no-rogue-finalizer_test_iteration
  timeout --foreground 5s bash -c "until no-rogue-finalizer_test_iteration; do sleep 1; done"
  # use --foreground to catch ctrl-c
  # https://unix.stackexchange.com/a/233685
}

no-rogue-finalizer_test_iteration() {
  set -euo pipefail
  source test/e2e/aliases.sh

  # check that we didn't add finalizers to uncontrolled resources
  finalizer="multicluster.admiralty.io/"
  for resource in pods configmaps secrets services ingresses; do
    [ $(k 1 get $resource -A -o custom-columns=FINALIZERS:.metadata.finalizers | grep -c $finalizer) -eq 0 ] || exit 1
  done
}

if [[ "${BASH_SOURCE[0]:-}" == "${0}" ]]; then
  no-rogue-finalizer_test "${@}"
fi
