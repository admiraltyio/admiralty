set -eu

# TODO check that the commented commands below don't change any files (check their output, or git status after)
# go run vendor/sigs.k8s.io/controller-tools/cmd/controller-gen/main.go all
# go generate ./pkg/... ./cmd/...
# go fmt ./pkg/... ./cmd/...

go vet ./pkg/... ./cmd/...
go test -v ./pkg/... ./cmd/... # -coverprofile cover.out
# TODO save cover.out somewhere
