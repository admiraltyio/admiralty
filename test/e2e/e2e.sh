set -euo pipefail

test/e2e/setup_clusters.sh
test/e2e/setup.sh
test/e2e/test.sh
test/e2e/tear_down.sh
test/e2e/tear_down_clusters.sh
