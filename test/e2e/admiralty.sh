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

VERSION="${VERSION:-dev}"

source test/e2e/aliases.sh
source test/e2e/kind.sh
source test/e2e/webhook_ready.sh

admiralty_setup() {
  i=$1
  VALUES=$2

  kind load docker-image multicluster-scheduler-agent:$VERSION-amd64 --name cluster$i
  kind load docker-image multicluster-scheduler-scheduler:$VERSION-amd64 --name cluster$i
  kind load docker-image multicluster-scheduler-remove-finalizers:$VERSION-amd64 --name cluster$i
  kind load docker-image multicluster-scheduler-restarter:$VERSION-amd64 --name cluster$i

  if ! k $i get ns admiralty; then
    k $i create namespace admiralty
  fi
  h $i upgrade --install multicluster-scheduler charts/multicluster-scheduler -n admiralty -f $VALUES \
    --set controllerManager.replicas=2 \
    --set scheduler.replicas=2 \
    --set restarter.replicas=2 \
    --set controllerManager.image.repository=multicluster-scheduler-agent \
    --set scheduler.image.repository=multicluster-scheduler-scheduler \
    --set postDeleteJob.image.repository=multicluster-scheduler-remove-finalizers \
    --set restarter.image.repository=multicluster-scheduler-restarter \
    --set controllerManager.image.tag=$VERSION-amd64 \
    --set scheduler.image.tag=$VERSION-amd64 \
    --set postDeleteJob.image.tag=$VERSION-amd64 \
    --set restarter.image.tag=$VERSION-amd64
  k $i delete pod --all -n admiralty

  k $i label ns default multicluster-scheduler=enabled --overwrite
}

if [[ "${BASH_SOURCE[0]:-}" == "${0}" ]]; then
  admiralty_setup "${@}"
fi
