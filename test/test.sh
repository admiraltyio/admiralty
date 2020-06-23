#!/usr/bin/env bash
set -euo pipefail

./hack/verify-codegen.sh
go vet ./pkg/... ./cmd/...
go test -v ./pkg/... ./cmd/... # -coverprofile cover.out
# TODO save cover.out somewhere
