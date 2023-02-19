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

  h $i upgrade --install multicluster-scheduler charts/multicluster-scheduler -n admiralty --create-namespace -f $VALUES \
    --set controllerManager.image.repository=multicluster-scheduler-agent \
    --set scheduler.image.repository=multicluster-scheduler-scheduler \
    --set postDeleteJob.image.repository=multicluster-scheduler-remove-finalizers \
    --set restarter.image.repository=multicluster-scheduler-restarter \
    --set controllerManager.image.tag=$VERSION-amd64 \
    --set scheduler.image.tag=$VERSION-amd64 \
    --set postDeleteJob.image.tag=$VERSION-amd64 \
    --set restarter.image.tag=$VERSION-amd64
  k $i delete pod --all -n admiralty
}

admiralty_connect() {
  i=$1
  j=$2
  ns="${3:-default}"

  if ! k "$i" get ns other; then
    k "$i" create ns other
  fi
  k $i label ns $ns multicluster-scheduler=enabled --overwrite

  if [[ $i == $j ]]; then
    # if self target
    cat <<EOF | k $i apply -f -
kind: Target
apiVersion: multicluster.admiralty.io/v1alpha1
metadata:
  name: c$j
  namespace: $ns
spec:
  self: true
EOF
  else
    if k $j cluster-info; then
      # if cluster j exists

      if ! k "$j" get ns other; then
        k "$j" create ns other
      fi

      cat <<EOF | k $j apply -f -
kind: Source
apiVersion: multicluster.admiralty.io/v1alpha1
metadata:
  name: cluster$i
  namespace: $ns
spec:
  serviceAccountName: cluster$i
---
apiVersion: v1
kind: Secret
metadata:
  name: cluster$i
  namespace: $ns
  annotations:
    kubernetes.io/service-account.name: cluster$i
type: kubernetes.io/service-account-token
EOF
      while ! k $j get secret cluster$i -n $ns -o json | jq -e .data.token; do sleep 1; done

      TOKEN=$(k $j get secret cluster$i -n $ns -o json | jq -r .data.token | base64 --decode)
      KUBECONFIG=$(k $j config view --minify --raw -o json | jq '.users[0].user={token:"'$TOKEN'"} | .contexts[0].context.namespace="default"')
      k $i create secret generic c$j -n $ns --from-literal=config="$KUBECONFIG" --dry-run -o yaml | k $i apply -f -
    fi

    # if cluster j doesn't exist, this is a misconfigured target
    # which must be handled gracefully
    cat <<EOF | k $i apply -f -
kind: Target
apiVersion: multicluster.admiralty.io/v1alpha1
metadata:
  name: c$j
  namespace: $ns
spec:
  kubeconfigSecret:
    name: c$j
EOF
  fi
}

if [[ "${BASH_SOURCE[0]:-}" == "${0}" ]]; then
  admiralty_setup "${@}"
fi
