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

BEFORE_VERSION="$1"
AFTER_VERSION="$2"

sed_opt="-i"
regex_opt=""
#sed_opt="--quiet"
#regex_opt="p"

sed $sed_opt "s/--version $BEFORE_VERSION/--version $AFTER_VERSION/g$regex_opt" docs/quick_start.md
sed $sed_opt "s/:$BEFORE_VERSION/:$AFTER_VERSION/g$regex_opt" docs/quick_start.md
sed $sed_opt "s/--version $BEFORE_VERSION/--version $AFTER_VERSION/g$regex_opt" docs/operator_guide/installation.md
sed $sed_opt "s/^version: $BEFORE_VERSION$/version: $AFTER_VERSION/$regex_opt" charts/multicluster-scheduler/Chart.yaml
sed $sed_opt "s/^appVersion: $BEFORE_VERSION$/appVersion: $AFTER_VERSION/$regex_opt" charts/multicluster-scheduler/Chart.yaml
sed $sed_opt "s/$BEFORE_VERSION/$AFTER_VERSION/g$regex_opt" charts/multicluster-scheduler/README.md
