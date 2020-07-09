#!/usr/bin/env bash
set -euo pipefail

DEFAULT_REGISTRY="quay.io/admiralty"

VERSION="$1"
REGISTRY="${2:-$DEFAULT_REGISTRY}"

CMDS=(
  "agent"
  "remove-finalizers"
  "scheduler"
  "restarter"
)

for CMD in "${CMDS[@]}"; do
  IMG="multicluster-scheduler-$CMD"
  docker tag "$DEFAULT_REGISTRY/$IMG:$VERSION" "$REGISTRY/$IMG:$VERSION"
  docker push "$REGISTRY/$IMG:$VERSION"
done

# TODO: also tag images with latest
