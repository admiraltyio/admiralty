#!/usr/bin/env bash
set -euo pipefail

VERSION="$1"

source test/e2e/mcsa.sh
install_kubemcsa

source test/e2e/cert-manager.sh
source test/e2e/clusters.sh $VERSION
source test/e2e/test_argo.sh

for T in single-namespace with-mcsa; do
  setup_clusters
  setup_argo
  setup_cert_manager
  source test/e2e/$T/test.sh
  setup
  test_blog_scenario_a_multicluster
  tear_down
  tear_down_cert_manager
  tear_down_argo
  tear_down_clusters
done

echo "ALL SUCCEEDED"
