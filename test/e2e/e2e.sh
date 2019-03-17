set -euo pipefail

test/e2e/setup_clusters.sh
test/e2e/cilium.sh
test/e2e/setup.sh
test/e2e/test_networking.sh
test/e2e/test_argo.sh
test/e2e/tear_down.sh
test/e2e/tear_down_clusters.sh
