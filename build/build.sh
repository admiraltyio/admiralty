#!/usr/bin/env bash
set -euo pipefail

VERSION="$1"

ROOT_PKG="admiralty.io/multicluster-scheduler"
TARGETS=(
  "cmd/agent"
  "cmd/pod-admission-controller"
  "cmd/scheduler"
)
IMAGES=(
  "multicluster-scheduler-agent"
  "multicluster-scheduler-pod-admission-controller"
  "multicluster-scheduler-basic"
)

for TARGET in "${TARGETS[@]}"; do
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o "_out/$TARGET/manager" "$ROOT_PKG/$TARGET"
done

cp build/Dockerfile _out/

for ((i = 0; i < ${#TARGETS[@]}; ++i)); do
  docker build -t "quay.io/admiralty/${IMAGES[i]}:$VERSION" --build-arg target="${TARGETS[i]}" _out
done
