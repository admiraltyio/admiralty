#!/usr/bin/env bash
set -euo pipefail

VERSION="$1"

IMAGES=(
  "multicluster-scheduler-agent"
  "multicluster-scheduler-pod-admission-controller"
  "multicluster-scheduler-basic"
)

for IMAGE in "${IMAGES[@]}"; do
  docker push "quay.io/admiralty/$IMAGE:$VERSION"
done

# TODO: upload Helm chart
# TODO: also tag images with latest
