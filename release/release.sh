#!/usr/bin/env bash
set -euo pipefail

VERSION="$1"

IMAGES=(
  "multicluster-scheduler-agent"
  "multicluster-scheduler-basic"
  "multicluster-scheduler-remove-finalizers"
)

for IMAGE in "${IMAGES[@]}"; do
  docker push "quay.io/admiralty/$IMAGE:$VERSION"
done

helm package charts/multicluster-scheduler -d _out
cp charts/index.yaml _out/
helm repo index _out --merge _out/index.yaml --url https://charts.admiralty.io

# TODO: upload Helm chart and new index
# TODO: also tag images with latest
