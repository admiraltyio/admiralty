set -euo pipefail

source test/e2e/aliases.sh

install_bootstrap_multicluster_service_account() {
	# Install MCSA and bootstrap cluster1 and cluster2 to import from cluster1 (which will host the control plane)
	RELEASE_URL=https://github.com/admiraltyio/multicluster-service-account/releases/download/v0.3.0
	MCSA_URL="$RELEASE_URL/install.yaml"
	k1 apply -f "$MCSA_URL"
	k2 apply -f "$MCSA_URL"
	# TODO: don't assume the right kubemcsa is installed
	kubemcsa bootstrap cluster1 cluster1
	kubemcsa bootstrap cluster2 cluster1
}

install_multicluster_scheduler() {
	c1 && skaffold run -f test/e2e/scheduler/skaffold.yaml
	kustomize build test/e2e/federation | k1 apply -f -
	c1 && skaffold run -f test/e2e/agent1/skaffold.yaml

	c2 && skaffold run -f test/e2e/agent2/skaffold.yaml
	# TODO: skaffold deploy rather than run, because images have already been built for cluster1 (need to pass tagged image names)

	# switch back to cluster1 for user commands afterward
	# TODO: save current context at beginning and return to it
	c1
}

install_bootstrap_multicluster_service_account
install_multicluster_scheduler
