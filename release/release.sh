#!/usr/bin/env bash
set -euo pipefail

VERSION="$1"
REGISTRY="${2:-quay.io/admiralty}"

CMDS=(
  "agent"
  "remove-finalizers"
  "scheduler"
)

for CMD in "${CMDS[@]}"; do
  IMG="multicluster-scheduler-$CMD"
  docker push "$REGISTRY/$IMG:$VERSION"
done

helm package charts/multicluster-scheduler -d _out
cp charts/index.yaml _out/
helm repo index _out --merge _out/index.yaml --url https://charts.admiralty.io

# TODO: upload Helm chart and new index
# TODO: also tag images with latest
# create release on GitHub
