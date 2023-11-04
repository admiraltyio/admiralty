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

# constants
default_registry="public.ecr.aws/admiralty"

# environment variables
# required
VERSION="${VERSION}"

# delete any leftover packaged charts
# (otherwise dates in index for their versions would be modified)
rm -f _out/admiralty-*.tgz

helm package charts/multicluster-scheduler -d _out

aws ecr-public get-login-password --region us-east-1 | helm registry login --username AWS --password-stdin public.ecr.aws

helm push _out/admiralty-"${VERSION}".tgz oci://$default_registry
