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

helm package charts/multicluster-scheduler -d _out
curl -s https://charts.admiralty.io/index.yaml >_out/index_old.yaml
helm repo index _out --merge _out/index_old.yaml --url https://charts.admiralty.io

# TODO: revert datetime created override for old versions (submit GitHub issue)

# release CRDs separately, esp. for `helm upgrade`
cat charts/multicluster-scheduler/crds/* >_out/admiralty.crds.yaml

# TODO: upload Helm chart and new index (with no-cache, max-age=0)
