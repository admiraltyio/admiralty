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

# constants
root_package=admiralty.io/multicluster-scheduler

# environment variables
# optional
VERSION="${VERSION:-dev}"
CR="${CR:-autra-cn-beijing.cr.volces.com/autra}"

if [ "$VERSION" = dev ] || [ "${CI:-}" = true ]; then
  linux_archs=(amd64)
else
  linux_archs=(amd64 arm64)
fi

for arch in "${linux_archs[@]}"; do
  export ARCH=$arch VERSION=$VERSION CR=$CR
  PKG=$root_package/cmd/agent ./build/build_one.sh
  PKG=$root_package/cmd/remove-finalizers ./build/build_one.sh
  PKG=$root_package/cmd/restarter ./build/build_one.sh
  PKG=$root_package/cmd/scheduler ./build/build_one.sh
  unset ARCH
done
