set -euo pipefail

ROOT_PKG="admiralty.io/multicluster-scheduler"
TARGETS=("cmd/agent" "cmd/pod-admission-controller" "cmd/scheduler")

for TARGET in "${TARGETS[@]}"; do
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o "_out/$TARGET/manager" "$ROOT_PKG/$TARGET"
done

cp build/Dockerfile _out/
