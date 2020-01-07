#!/usr/bin/env bash
set -euo pipefail

source test/e2e/aliases.sh

webhook_ready() {
  cluster_id=$1
  namespace=$2
  deployment_name=$3
  config_name=$4
  secret_name=$5

  echo "waiting for webhook deployment to be available..."
  k$cluster_id wait --for condition=available --timeout=120s deployment $deployment_name -n $namespace

  echo -n "waiting for webhook configuration CA bundle to match secret..."
  while :; do
    secret_cert=$(k$cluster_id get secret $secret_name -n $namespace -o json | jq -r '.data["ca.crt"]')
    webhook_cert=$(k$cluster_id get mutatingwebhookconfiguration $config_name -n $namespace -o json | jq -r .webhooks[0].clientConfig.caBundle)
    if [ "$secret_cert" == "$webhook_cert" ]; then
      echo
      break
    fi
    sleep 1
    echo -n "."
  done

  # still something is missing
  # maybe https://github.com/kubernetes-sigs/controller-runtime/issues/723
  # so for now...
  sleep 10
}
