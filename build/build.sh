#!/usr/bin/env bash
set -euo pipefail

VERSION="$1"
REGISTRY="${2:-quay.io/admiralty}"

ROOT_PKG="admiralty.io/multicluster-scheduler"
CMDS=(
  "agent"
  "remove-finalizers"
  "scheduler"
)

cp build/Dockerfile _out/

for CMD in "${CMDS[@]}"; do
  TARGET="cmd/$CMD"
  IMG="multicluster-scheduler-$CMD"
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o "_out/$TARGET/manager" "$ROOT_PKG/$TARGET"
  docker build -t "$REGISTRY/$IMG:$VERSION" --build-arg target="$TARGET" _out
done
