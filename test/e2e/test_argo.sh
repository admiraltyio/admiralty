set -euo pipefail

source test/e2e/aliases.sh

setup() {
	# Install Argo in cluster1
	k1 create ns argo
	k1 apply -n argo -f https://raw.githubusercontent.com/argoproj/argo/v2.2.1/manifests/install.yaml
	k1 apply -f config/samples/argo-workflows/_service-account.yaml
	# the workflow service account must exist in the other cluster
	k2 apply -f config/samples/argo-workflows/_service-account.yaml

	k1 label ns default multicluster-scheduler=enabled
}

tear_down() {
	k1 label ns default multicluster-scheduler-

	k2 delete -f config/samples/argo-workflows/_service-account.yaml
	k1 delete -f config/samples/argo-workflows/_service-account.yaml
	k1 delete -n argo -f https://raw.githubusercontent.com/argoproj/argo/v2.2.1/manifests/install.yaml
	k1 delete ns argo
}

test_blog_scenario_a_multicluster() {
	argo --context cluster1 submit --serviceaccount argo-workflow --watch config/samples/argo-workflows/blog-scenario-a-multicluster.yaml

	if [ $(k2 get pod | wc -l) -gt 1 ]
	then
		echo "SUCCESS"
		exit 0
	else
		echo "FAILURE"
		exit 1
	fi

	argo --context cluster1 delete --all
}

setup
test_blog_scenario_a_multicluster
tear_down
