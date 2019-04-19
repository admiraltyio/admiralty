# set -euo pipefail

source test/e2e/aliases.sh

c2 && skaffold delete -f test/e2e/agent2/skaffold.yaml
c1 && skaffold delete -f test/e2e/agent1/skaffold.yaml
kustomize build test/e2e/federation | k1 delete -f -
c1 && skaffold delete -f test/e2e/scheduler/skaffold.yaml

RELEASE_URL=https://github.com/admiraltyio/multicluster-service-account/releases/download/v0.3.1
MCSA_URL="$RELEASE_URL/install.yaml"
k2 delete -f "$MCSA_URL"
k1 delete -f "$MCSA_URL"
