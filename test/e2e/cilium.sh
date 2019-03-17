set -euo pipefail

source test/e2e/aliases.sh

setup() {
	git clone https://github.com/cilium/clustermesh-tools.git
	cd clustermesh-tools
	export NAMESPACE=cilium # for extract-etcd-secrets.sh (default: kube-system)

	for CLUSTER_ID in 1 2; do
		CLUSTER_NAME=cluster$CLUSTER_ID

		# from https://docs.cilium.io/en/v1.4/gettingstarted/k8s-install-gke/
		k$CLUSTER_ID create ns cilium

		k$CLUSTER_ID -n cilium apply -f https://raw.githubusercontent.com/cilium/cilium/v1.4.2/examples/kubernetes/node-init/node-init.yaml

		echo "waiting for daemon set cilium-node-init to be ready"
		until [ $(k$CLUSTER_ID -n cilium get ds cilium-node-init -o jsonpath="{.status.numberReady}") == 3 ]; do echo -n "."; sleep 1; done; echo

		echo "waiting for cilium-node-init to have completed successfully on all nodes" # redundant?
		until [ $(k$CLUSTER_ID -n cilium logs -l app=cilium-node-init 2>/dev/null | grep "startup-script succeeded" | wc -l) == 3 ]; do echo -n "."; sleep 1; done; echo

		echo "waiting for all nodes to be ready..."
		k$CLUSTER_ID wait node --for condition=ready -l beta.kubernetes.io/arch=amd64 --timeout=60s

		k$CLUSTER_ID -n kube-system delete pod -l k8s-app=kube-dns
		# note: kube-dns won't restart until cilium is available ("failed to find plugin cilium-cni")
		# however, cilium etcd checks DNS

		k$CLUSTER_ID apply -f https://raw.githubusercontent.com/cilium/cilium/v1.4.2/examples/kubernetes/1.11/cilium-with-node-init.yaml

		echo "waiting for daemon set cilium to be ready"
		until [ $(k$CLUSTER_ID -n cilium get ds cilium -o jsonpath="{.status.numberReady}") == 3 ]; do echo -n "."; sleep 1; done; echo

		echo "waiting for deployment cilium-operator to be available..."
		k$CLUSTER_ID -n cilium wait deploy/cilium-operator --for condition=available --timeout=60s

		echo "waiting for deployment cilium-etcd-operator to be available..."
		k$CLUSTER_ID -n cilium wait deploy/cilium-etcd-operator --for condition=available --timeout=60s

		echo "waiting for EtcdCluster cilium-etcd to be available..."
		k$CLUSTER_ID -n cilium wait EtcdCluster/cilium-etcd --for condition=available --timeout=60s

		echo "waiting for deployment kube-dns to be available..."
		k$CLUSTER_ID -n kube-system wait deploy/kube-dns --for condition=available --timeout=60s

		# restart pods stuck in crash loop (and prevent tear-down...)
		k$CLUSTER_ID -n kube-system delete pod -l k8s-app=heapster
		k$CLUSTER_ID -n kube-system delete pod -l k8s-app=glbc

		# from https://docs.cilium.io/en/v1.4/gettingstarted/clustermesh/
		k$CLUSTER_ID -n cilium patch cm cilium-config -p '{"data":{"cluster-id": "'$CLUSTER_ID'", "cluster-name": "'$CLUSTER_NAME'"}}'
		k$CLUSTER_ID -n cilium apply -f https://raw.githubusercontent.com/cilium/cilium/v1.4.2/examples/kubernetes/clustermesh/cilium-etcd-external-service/cilium-etcd-external-gke.yaml

		echo "waiting for secret cilium-etcd-secrets to exist"
		until k$CLUSTER_ID -n cilium get secret "cilium-etcd-secrets" &> /dev/null; do echo -n "."; sleep 1; done; echo

		echo "waiting for service cilium-etcd-external to have an load balancer ingress IP"
		while [ -z "$(k$CLUSTER_ID -n cilium get svc cilium-etcd-external -o jsonpath="{.status.loadBalancer.ingress[0].ip}")" ]; do echo -n "."; sleep 1; done; echo

		c$CLUSTER_ID && ./extract-etcd-secrets.sh
	done

	./generate-secret-yaml.sh > clustermesh.yaml
	./generate-name-mapping.sh > ds.patch

	for CLUSTER_ID in 1 2; do
		CLUSTER_NAME=cluster$CLUSTER_ID

		k$CLUSTER_ID -n cilium patch ds cilium -p "$(cat ds.patch)"
		k$CLUSTER_ID -n cilium apply -f clustermesh.yaml
		k$CLUSTER_ID -n cilium delete pod -l k8s-app=cilium

		echo "waiting for daemon set cilium to be ready"
		until [ $(k$CLUSTER_ID -n cilium get ds cilium -o jsonpath="{.status.numberReady}") == 3 ]; do echo -n "."; sleep 1; done; echo

		# missing step in doc: cilium-operator must be restarted
		# "the cilium-operator deployment [...] is responsible to propagate Kubernetes services into the kvstore"
		# TODO: cilium/cilium PR
		k$CLUSTER_ID -n cilium delete pod -l name=cilium-operator

		echo "waiting for deployment cilium-operator to be available..."
		k$CLUSTER_ID -n cilium wait deploy/cilium-operator --for condition=available --timeout=60s

		k$CLUSTER_ID apply -f https://raw.githubusercontent.com/cilium/cilium/v1.4.2/examples/kubernetes/clustermesh/global-service-example/$CLUSTER_NAME.yaml

		echo "waiting for deployment rebel-base to roll out..."
		k$CLUSTER_ID rollout status deploy/rebel-base

		echo "waiting for deployment x-wing to roll out..."
		k$CLUSTER_ID rollout status deploy/x-wing
	done

	cd ..

	rm -r clustermesh-tools
}

test() {
	CLUSTER1_REBEL_BASE_POD_IP=$(k1 get pod -l name=rebel-base -o jsonpath={.items[0].status.podIP})
	echo "CLUSTER1_REBEL_BASE_POD_IP=$CLUSTER1_REBEL_BASE_POD_IP"
	CLUSTER2_REBEL_BASE_POD_IP=$(k2 get pod -l name=rebel-base -o jsonpath={.items[0].status.podIP})
	echo "CLUSTER2_REBEL_BASE_POD_IP=$CLUSTER2_REBEL_BASE_POD_IP"

	CLUSTER1_X_WING_POD_NAME=$(k1 get pod -l name=x-wing -o jsonpath={.items[0].metadata.name})
	echo "CLUSTER1_X_WING_POD_NAME=$CLUSTER1_X_WING_POD_NAME"
	CLUSTER2_X_WING_POD_NAME=$(k2 get pod -l name=x-wing -o jsonpath={.items[0].metadata.name})
	echo "CLUSTER2_X_WING_POD_NAME=$CLUSTER2_X_WING_POD_NAME"

	k1 exec $CLUSTER1_X_WING_POD_NAME -- curl -s $CLUSTER2_REBEL_BASE_POD_IP
	k2 exec $CLUSTER2_X_WING_POD_NAME -- curl -s $CLUSTER1_REBEL_BASE_POD_IP

	echo "calling from cluster1"
	Cluster=""
	until [ "$Cluster" == "Cluster-2" ]; do
		Cluster=$(k1 exec $CLUSTER1_X_WING_POD_NAME -- curl -s rebel-base | jq -r .Cluster)
		echo $Cluster
	done

	echo "calling from cluster2"
	Cluster=""
	until [ "$Cluster" == "Cluster-1" ]; do
		Cluster=$(k2 exec $CLUSTER2_X_WING_POD_NAME -- curl -s rebel-base | jq -r .Cluster)
		echo $Cluster
	done
}

# tear_down() {

# }

setup
test
# TODO: tear down
