#!/usr/bin/env bash
#
# Copyright 2021 The Multicluster-Scheduler Authors.
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

REGION="${REGION:-"us-west-2"}"
VERSION="${VERSION:-dev}"

eks_setup_once() {
  aws_account_id="$(aws sts get-caller-identity | jq -r .Account)"

  registry="$aws_account_id.dkr.ecr.$REGION.amazonaws.com"
  if [[ "${CI:-}" != true ]]; then
    aws ecr get-login-password | docker login --username AWS --password-stdin "$registry"
    # TODO... use https://github.com/awslabs/amazon-ecr-credential-helper
  fi

  imgs=(
    multicluster-scheduler-agent
    multicluster-scheduler-remove-finalizers
    multicluster-scheduler-scheduler
    multicluster-scheduler-restarter
  )

  for img in "${imgs[@]}"; do
    if ! aws ecr describe-repositories --region $REGION --repository-names $img; then
      aws ecr create-repository --region $REGION --repository-name $img
    fi
    ARCHS=amd64 VERSION="${VERSION}" REGISTRY=$registry IMG="$img" ./release/image.sh
  done

  K8S_VERSION="${K8S_VERSION:-"1.21"}"
}

eks_setup() {
  i=$1

  cluster=cluster$i
  kubeconfig="kubeconfig-$cluster"
  ekscluster=$cluster-${K8S_VERSION//./-} # for concurrency

  if ! eksctl get cluster --name $ekscluster --region $REGION; then
    eksctl create cluster --name $ekscluster --region $REGION --managed --kubeconfig $kubeconfig --version $K8S_VERSION
  fi
}

if [[ "${BASH_SOURCE[0]:-}" == "${0}" ]]; then
  eks_setup_once
  eks_setup "${@}"
fi
