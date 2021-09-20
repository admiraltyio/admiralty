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
REGISTRY="${REGISTRY:-}"

source test/e2e/aliases.sh

admiralty_setup() {
  i=$1
  VALUES=$2

  if ! k $i get ns admiralty; then
    k $i create namespace admiralty
  fi
  img_prefix=""
  pull_policy=IfNotPresent
  if [[ "$REGISTRY" != "" ]]; then
    img_prefix="$REGISTRY/"
    pull_policy=Always
  fi
  h $i upgrade --install multicluster-scheduler charts/multicluster-scheduler -n admiralty -f $VALUES \
    --set controllerManager.image.repository=${img_prefix}multicluster-scheduler-agent \
    --set scheduler.image.repository=${img_prefix}multicluster-scheduler-scheduler \
    --set postDeleteJob.image.repository=${img_prefix}multicluster-scheduler-remove-finalizers \
    --set restarter.image.repository=${img_prefix}multicluster-scheduler-restarter \
    --set controllerManager.image.pullPolicy=$pull_policy \
    --set scheduler.image.pullPolicy=$pull_policy \
    --set postDeleteJob.image.pullPolicy=$pull_policy \
    --set restarter.image.pullPolicy=$pull_policy \
    --set controllerManager.image.tag=$VERSION-amd64 \
    --set scheduler.image.tag=$VERSION-amd64 \
    --set postDeleteJob.image.tag=$VERSION-amd64 \
    --set restarter.image.tag=$VERSION-amd64
  k $i delete pod --all -n admiralty

  k $i label ns default multicluster-scheduler=enabled --overwrite
}

admiralty_connect() {
  i=$1
  j=$2

  if [[ $i == $j ]]; then
    # if self target
    cat <<EOF | k $i apply -f -
kind: Target
apiVersion: multicluster.admiralty.io/v1alpha1
metadata:
  name: c$j
spec:
  self: true
EOF
  else
    if k $j cluster-info; then
      # if cluster j exists
      cat <<EOF | k $j apply -f -
kind: Source
apiVersion: multicluster.admiralty.io/v1alpha1
metadata:
  name: cluster$i
spec:
  serviceAccountName: cluster$i
EOF
      while ! k $j get sa cluster$i; do sleep 1; done

      SECRET_NAME=$(k $j get serviceaccount cluster1 -o json | jq -r .secrets[0].name)
      TOKEN=$(k $j get secret $SECRET_NAME -o json | jq -r .data.token | base64 --decode)
      KUBECONFIG=$(k $j config view --minify --raw -o json | jq '.users[0].user={token:"'$TOKEN'"} | .contexts[0].context.namespace="default"')
      k $i create secret generic c$j --from-literal=config="$KUBECONFIG" --dry-run -o yaml | k $i apply -f -
    fi

    # if cluster j doesn't exist, this is a misconfigured target
    # which must be handled gracefully
    cat <<EOF | k $i apply -f -
kind: Target
apiVersion: multicluster.admiralty.io/v1alpha1
metadata:
  name: c$j
spec:
  kubeconfigSecret:
    name: c$j
EOF
  fi
}

if [[ "${BASH_SOURCE[0]:-}" == "${0}" ]]; then
  admiralty_setup "${@}"
fi
