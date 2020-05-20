#!/usr/bin/env bash
set -euo pipefail

helm package charts/multicluster-scheduler -d _out
cp charts/index.yaml _out/
helm repo index _out --merge _out/index.yaml --url https://charts.admiralty.io

# TODO: upload Helm chart and new index
