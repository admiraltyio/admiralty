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
# TODO: migrate to k8s.io/code-generator/kube_codegen.sh

set -o errexit
set -o nounset
set -o pipefail

GOPATH="${GOPATH:-"$HOME/go"}"

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
CODEGEN_PKG=${CODEGEN_PKG:-$(
  cd "${SCRIPT_ROOT}"
  ls -d -1 $GOPATH/pkg/mod/k8s.io/code-generator@v0.27.4 2>/dev/null || echo ../code-generator
)}

# generate the code with:
# --output-base    because this script should also be able to run inside the vendor dir of
#                  k8s.io/kubernetes. The output-base is needed for the generators to output into the vendor dir
#                  instead of the $GOPATH directly. For normal projects this can be dropped.
GOPATH=$GOPATH bash "${CODEGEN_PKG}"/generate-groups.sh "deepcopy,client,informer,lister" \
  admiralty.io/multicluster-scheduler/pkg/generated admiralty.io/multicluster-scheduler/pkg/apis \
  multicluster:v1alpha1 \
  --go-header-file "${SCRIPT_ROOT}"/hack/boilerplate.go.txt

GOPATH=$GOPATH bash "${CODEGEN_PKG}"/generate-internal-groups.sh "deepcopy,conversion,defaulter" \
  admiralty.io/multicluster-scheduler/pkg/generated \
  admiralty.io/multicluster-scheduler/pkg/apis \
  admiralty.io/multicluster-scheduler/pkg/apis \
  "config:v1" \
  --trim-path-prefix admiralty.io/multicluster-scheduler/ \
  --go-header-file "${SCRIPT_ROOT}"/hack/boilerplate.go.txt

# To use your own boilerplate text append:
#   --go-header-file "${SCRIPT_ROOT}"/hack/custom-boilerplate.go.txt
