#!/usr/bin/env bash
#
# Copyright 2020 The Multicluster-Scheduler Authors.
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

ingress_test() {
  i=$1
  j=$2

  k $i apply -f test/e2e/ingress/test.yaml
  sleep 10
  [ $(k "$j" get ingress | wc -l) -eq 2 ] # including header
  [ $(k "$j" get service | wc -l) -eq 3 ] # including header and the "kubernetes" service
  k $i delete -f test/e2e/ingress/test.yaml
}

if [[ "${BASH_SOURCE[0]:-}" == "${0}" ]]; then
  ingress_test "${@}"
fi
