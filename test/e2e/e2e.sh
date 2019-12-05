#!/usr/bin/env bash
set -euo pipefail

VERSION="$1"

source test/e2e/mcsa.sh
install_kubemcsa

source test/e2e/clusters.sh $VERSION
source test/e2e/test_argo.sh

for T in single-namespace cluster-namespaces with-mcsa; do # TODO multi-federation
  setup_clusters
  setup_argo
  source test/e2e/$T/test.sh
  setup
  test_blog_scenario_a_multicluster
  tear_down
  tear_down_argo
  tear_down_clusters
done
