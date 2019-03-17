set -euo pipefail

source test/e2e/aliases.sh

setup() {
	k1 label ns default multicluster-scheduler=enabled

	c1 && skaffold run -f test/e2e/networking/cluster1/skaffold.yaml
	c2 && skaffold run -f test/e2e/networking/cluster2/skaffold.yaml

	k1 rollout status deploy/global-sample-service-cluster1
	k1 rollout status deploy/mc-sample-service-cluster1
	k1 rollout status deploy/mc-sample-service-cluster2
	k1 rollout status deploy/sample-service-cluster1

	k2 rollout status deploy/global-sample-service-cluster2
	k2 rollout status deploy/sample-service-cluster2
}

test() {
	CLUSTER1_DELEGATE_MC_SAMPLE_SERVICE_POD_NAME=$(k1 get pod -l multicluster.admiralty.io/app=mc-sample-service-cluster1 -o jsonpath={.items[0].metadata.name})
	echo "CLUSTER1_DELEGATE_MC_SAMPLE_SERVICE_POD_NAME=$CLUSTER1_DELEGATE_MC_SAMPLE_SERVICE_POD_NAME"

	CLUSTER1_GLOBAL_SAMPLE_SERVICE_POD_NAME=$(k1 get pod -l app=global-sample-service-cluster1 -o jsonpath={.items[0].metadata.name})
	echo "CLUSTER1_GLOBAL_SAMPLE_SERVICE_POD_NAME=$CLUSTER1_GLOBAL_SAMPLE_SERVICE_POD_NAME"

	CLUSTER1_SAMPLE_SERVICE_POD_NAME=$(k1 get pod -l app=sample-service-cluster1 -o jsonpath={.items[0].metadata.name})
	echo "CLUSTER1_SAMPLE_SERVICE_POD_NAME=$CLUSTER1_SAMPLE_SERVICE_POD_NAME"

	CLUSTER2_DELEGATE_MC_SAMPLE_SERVICE_POD_NAME=$(k2 get pod -l multicluster.admiralty.io/app=mc-sample-service-cluster2 -o jsonpath={.items[0].metadata.name})
	echo "CLUSTER2_DELEGATE_MC_SAMPLE_SERVICE_POD_NAME=$CLUSTER2_DELEGATE_MC_SAMPLE_SERVICE_POD_NAME"

	CLUSTER2_GLOBAL_SAMPLE_SERVICE_POD_NAME=$(k2 get pod -l app=global-sample-service-cluster2 -o jsonpath={.items[0].metadata.name})
	echo "CLUSTER2_GLOBAL_SAMPLE_SERVICE_POD_NAME=$CLUSTER2_GLOBAL_SAMPLE_SERVICE_POD_NAME"

	CLUSTER2_SAMPLE_SERVICE_POD_NAME=$(k2 get pod -l app=sample-service-cluster2 -o jsonpath={.items[0].metadata.name})
	echo "CLUSTER2_SAMPLE_SERVICE_POD_NAME=$CLUSTER2_SAMPLE_SERVICE_POD_NAME"

	for pod_name in "$CLUSTER1_DELEGATE_MC_SAMPLE_SERVICE_POD_NAME" "$CLUSTER1_GLOBAL_SAMPLE_SERVICE_POD_NAME" "$CLUSTER1_SAMPLE_SERVICE_POD_NAME"; do
		[ "$(k1 exec $pod_name -- curl -s global-sample-service-cluster1)" == "$CLUSTER1_GLOBAL_SAMPLE_SERVICE_POD_NAME" ]
		echo $?
		[ "$(k1 exec $pod_name -- curl -s global-sample-service-cluster2)" == "$CLUSTER2_GLOBAL_SAMPLE_SERVICE_POD_NAME" ]
		echo $?
		[ "$(k1 exec $pod_name -- curl -s mc-sample-service-cluster1)" == "$CLUSTER1_DELEGATE_MC_SAMPLE_SERVICE_POD_NAME" ]
		echo $?
		[ "$(k1 exec $pod_name -- curl -s mc-sample-service-cluster2)" == "$CLUSTER2_DELEGATE_MC_SAMPLE_SERVICE_POD_NAME" ]
		echo $?
		[ "$(k1 exec $pod_name -- curl -s sample-service-cluster1)" == "$CLUSTER1_SAMPLE_SERVICE_POD_NAME" ]
		echo $?
	done

	for pod_name in "$CLUSTER2_DELEGATE_MC_SAMPLE_SERVICE_POD_NAME" "$CLUSTER2_GLOBAL_SAMPLE_SERVICE_POD_NAME" "$CLUSTER2_SAMPLE_SERVICE_POD_NAME"; do
		[ "$(k2 exec $pod_name -- curl -s global-sample-service-cluster1)" == "$CLUSTER1_GLOBAL_SAMPLE_SERVICE_POD_NAME" ]
		echo $?
		[ "$(k2 exec $pod_name -- curl -s global-sample-service-cluster2)" == "$CLUSTER2_GLOBAL_SAMPLE_SERVICE_POD_NAME" ]
		echo $?
		[ "$(k2 exec $pod_name -- curl -s mc-sample-service-cluster1)" == "$CLUSTER1_DELEGATE_MC_SAMPLE_SERVICE_POD_NAME" ]
		echo $?
		[ "$(k2 exec $pod_name -- curl -s mc-sample-service-cluster2)" == "$CLUSTER2_DELEGATE_MC_SAMPLE_SERVICE_POD_NAME" ]
		echo $?
		[ "$(k2 exec $pod_name -- curl -s sample-service-cluster2)" == "$CLUSTER2_SAMPLE_SERVICE_POD_NAME" ]
		echo $?
	done
}

tear_down() {
	c1 && skaffold delete -f test/e2e/networking/cluster1/skaffold.yaml
	c2 && skaffold delete -f test/e2e/networking/cluster2/skaffold.yaml

	k1 label ns default multicluster-scheduler-
}

setup
test
tear_down
