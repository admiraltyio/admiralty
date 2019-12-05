#!/usr/bin/env bash
set -euo pipefail

# TODO check that hack/codegen.sh doesn't change any files (check output or git status afterward)

go vet ./pkg/... ./cmd/...
go test -v ./pkg/... ./cmd/... # -coverprofile cover.out
# TODO save cover.out somewhere
