# Multicluster-Scheduler

Multicluster-scheduler is a system of Kubernetes controllers—a scheduler and its agents—that intelligently schedules workloads across clusters. It differs from [Federation v2](https://github.com/kubernetes-sigs/federation-v2) in several ways:

- Multi-cluster workloads can be declared in any member cluster and/or the scheduler's control plane (the Kubernetes API of the cluster hosting the scheduler), allowing greater architectural flexibility–e.g., don't give users direct access to the scheduler's cluster.
- The agents push observations and pull scheduling decisions to/from the scheduler's control plane. The scheduler reconciles scheduling decisions with observations, but never calls the Kubernetes APIs of the member clusters.

Multicluster-scheduler implements a basic spread scheduler, which can be replaced by more advanced implementations leveraging pod, node, and node-pool observations. For example, [Admiralty's hybrid and multicloud scheduler as a service](https://admiralty.io), built upon multicluster-scheduler, supports strategies including burst-to-cloud and real-time arbitrage (cost optimization).

## How it Works

![](doc/multicluster-scheduler-sequence-diagram.svg)

Multicluster-scheduler includes custom resource definitions (CRDs) for:

- **node pools**, which hold min/max node counts and pricing information;
- **multi-cluster workloads**, e.g., multi-cluster deployments, multicluster-scheduler's user-facing API;
- **observations**, images of observed node pools, nodes, pods, and multi-cluster workloads;
- and **decisions**, images of desired single-cluster workloads.

Note: For now, multicluster-scheduler defines its own multi-cluster workload API, but we're considering integrations with Federation v2's API and/or [Crossplane](https://crossplane.io).

Node pools and multi-cluster workloads are defined in each member cluster, whereas observations and decisions are only defined in the scheduler's control plane.

The custom resources are controlled by two managers:

- The **agent**, deployed in each member cluster, manages three controllers:
    - The **node pool controller** automatically creates node pool objects in the agent's cluster. In GKE and AKS, it uses the `cloud.google.com/gke-nodepool` or `agentpool` label, respectively; in the absence of those labels, a default node pool object is created. Min/max node counts and pricing information can be updated by the user, or controlled by other tools. Custom node pool objects can also be created using label selectors.
    - The **observations controller**, a [multi-cluster controller](https://github.com/admiraltyio/multicluster-controller), watches node pools, nodes, pods, and multi-cluster workloads in the agent's cluster and reconciles corresponding observations in the scheduler's control plane.
    - The **decisions controller**, another multi-cluster controller, watches scheduling decisions in the scheduler's control plane and reconciles corresponding single-cluster workloads in the agent's cluster.
- The **scheduler**, deployed wherever, manages an eponymous controller that watches observations and makes scheduling decisions in the scheduler's cluster. For example, it creates one or several deployment decisions from a multi-cluster deployment observation, using node pool, node and pod observations to inform those decisions. The scheduler doesn't push anything to the member clusters.

## Getting Started

We assume that you are a cluster admin for two clusters, associated with, e.g., the contexts "cluster1" and "cluster2" in your kubeconfig. We're going to install a basic scheduler in cluster1 and agents in cluster1 and cluster2, but the scripts can easily be adapted for other configurations. Then, we will deploy a multi-cluster NGINX.

```bash
CLUSTER1=cluster1 # change me
CLUSTER2=cluster2 # change me
```

Note: with [Admiralty's hybrid and multicloud scheduler as a service](https://admiralty.io), which is basically a managed scheduler with advanced features, you only need to install the agent and a Secret. To install the full multicluster-scheduler, please read on.

### Installation

#### Scheduler

Choose a cluster to host the scheduler, download the basic scheduler's manifest and install it:

```bash
SCHEDULER_CLUSTER_NAME="$CLUSTER1"
RELEASE_URL=https://github.com/admiraltyio/multicluster-scheduler/releases/download/v0.1.0
kubectl config use-context "$SCHEDULER_CLUSTER_NAME"
kubectl apply -f "$RELEASE_URL/scheduler.yaml"
```

#### Federation

In the same cluster as the scheduler, create a namespace for each cluster federation and, in it, a service account and role binding for each member cluster. The scheduler's cluster can be a member too.

```bash
FEDERATION_NAMESPACE=foo
kubectl create namespace "$FEDERATION_NAMESPACE"
MEMBER_CLUSTER_NAMES=("$CLUSTER1" "$CLUSTER2") # Add as many member clusters as you want.
for CLUSTER_NAME in "${MEMBER_CLUSTER_NAMES[@]}"; do
    kubectl create serviceaccount "$CLUSTER_NAME" \
        --namespace "$FEDERATION_NAMESPACE"
    kubectl create rolebinding "$CLUSTER_NAME" \
        --namespace "$FEDERATION_NAMESPACE" \
        --serviceaccount "$FEDERATION_NAMESPACE:$CLUSTER_NAME" \
        --clusterrole multicluster-scheduler-member
done
```

#### Multicluster-Service-Account

If you're already running [multicluster-service-account](https://github.com/admiraltyio/multicluster-service-account) in each member cluster, and the scheduler's cluster is known to them as `$SCHEDULER_CLUSTER_NAME`, you can skip this step and [install the agents](#agent). Otherwise, read on.

Download the multicluster-service-account manifest and install it in each member cluster:

```bash
MCSA_RELEASE_URL=https://github.com/admiraltyio/multicluster-service-account/releases/download/v0.2.0
for CLUSTER_NAME in "${MEMBER_CLUSTER_NAMES[@]}"; do
    kubectl --context "$CLUSTER_NAME" apply -f "$MCSA_RELEASE_URL/install.yaml"
done
```

Then, download the kubemcsa binary and run the bootstrap command to allow member clusters to import service accounts from the scheduler's cluster:

```bash
OS=linux # or darwin (i.e., OS X) or windows
ARCH=amd64 # if you're on a different platform, you must know how to build from source
curl -Lo kubemcsa "$MCSA_RELEASE_URL/kubemcsa-$OS-$ARCH"
chmod +x kubemcsa
sudo mv kubemcsa /usr/local/bin

for CLUSTER_NAME in "${MEMBER_CLUSTER_NAMES[@]}"; do
    kubemcsa bootstrap "$CLUSTER_NAME" "$SCHEDULER_CLUSTER_NAME"
done
```

#### Agent

Download the agent manifest and install it in each member cluster:

```bash
curl -LO $RELEASE_URL/agent.yaml
for CLUSTER_NAME in "${MEMBER_CLUSTER_NAMES[@]}"; do
    sed -e "s/SCHEDULER_CLUSTER_NAME/$SCHEDULER_CLUSTER_NAME/g" \
        -e "s/FEDERATION_NAMESPACE/$FEDERATION_NAMESPACE/g" \
        -e "s/CLUSTER_NAME/$CLUSTER_NAME/g" \
        agent.yaml > "agent-$CLUSTER_NAME.yaml"
    kubectl --context "$CLUSTER_NAME" apply -f "agent-$CLUSTER_NAME.yaml"
done
```

Check that node pool objects have been created in the agents' clusters and observations appear in the scheduler's control plane:

```bash
for CLUSTER_NAME in "${MEMBER_CLUSTER_NAMES[@]}"; do
    kubectl --context "$CLUSTER_NAME" get nodepools
done
kubectl --namespace "$FEDERATION_NAMESPACE" get nodepoolobservations
kubectl --namespace "$FEDERATION_NAMESPACE" get nodeobservations
kubectl --namespace "$FEDERATION_NAMESPACE" get podobservations
# or
kubectl --namespace "$FEDERATION_NAMESPACE" get observations # by category
```

### Example

Deploy NGINX as a MulticlusterDeployment in any of the member cluster, e.g., cluster2:

```bash
cat <<EOF | kubectl --context "$CLUSTER2" apply -f -
apiVersion: multicluster.admiralty.io/v1alpha1
kind: MulticlusterDeployment
metadata:
  name: nginx
spec:
  replicas: 5
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx
EOF
```

Check that an observation of the multi-cluster deployment has been created in the scheduler's control plane, as well as one deployment decision for each member cluster, hence one deployment in each member cluster:

```bash
kubectl --namespace "$FEDERATION_NAMESPACE" get multiclusterdeploymentobservations
kubectl --namespace "$FEDERATION_NAMESPACE" get deploymentdecisions
for CLUSTER_NAME in "${MEMBER_CLUSTER_NAMES[@]}"; do
    kubectl --context "$CLUSTER_NAME" get deployments
done
```

## Bring Your Own Scheduler

You can easily implement your own multi-cluster scheduler using the [Scheduler interface](https://godoc.org/admiralty.io/multicluster-scheduler/pkg/controllers/schedule#Scheduler). For now, only deployments are supported. Here's a basic manager scaffolding, where the custom `scheduler` struct, which implements the `Scheduler` interface, is passed as an argument to the controller:

```go
package main

import (
  "log"

  "admiralty.io/multicluster-controller/pkg/cluster"
  "admiralty.io/multicluster-controller/pkg/manager"
  "admiralty.io/multicluster-scheduler/pkg/apis/multicluster/v1alpha1"
  "admiralty.io/multicluster-scheduler/pkg/controllers/schedule"
  "admiralty.io/multicluster-service-account/pkg/config"
  appsv1 "k8s.io/api/apps/v1"
  corev1 "k8s.io/api/core/v1"
  _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
  "k8s.io/sample-controller/pkg/signals"
)

func main() {
  cfg, _, err := config.ConfigAndNamespace()
  if err != nil {
    log.Fatalf("cannot load config: %v", err)
  }
  cl := cluster.New("", cfg, cluster.Options{})

  co, err := schedule.NewController(cl, &scheduler{})
  if err != nil {
    log.Fatalf("cannot create scheduler controller: %v", err)
  }

  m := manager.New()
  m.AddController(co)

  if err := m.Start(signals.SetupSignalHandler()); err != nil {
    log.Fatalf("while or after starting manager: %v", err)
  }
}

type scheduler struct {
    // ... e.g., structured observations
}

func (s *scheduler) SetNodePool(np *v1alpha1.NodePool) {
    // ... e.g., store to inform scheduling decisions
    // Note: the controller has added a ClusterName to the object's metadata.
}

func (s *scheduler) SetNode(n *corev1.Node) {
    // ... e.g., store to inform scheduling decisions
    // Note: the controller has added a ClusterName to the object's metadata.
}
func (s *scheduler) SetPod(p *corev1.Pod) {
    // ... e.g., store to inform scheduling decisions
    // Note: the controller has added a ClusterName to the object's metadata.
}

func (s *scheduler) Schedule(mcd *v1alpha1.MulticlusterDeployment) ([]*appsv1.Deployment, error) {
    // ... Given a MulticlusterDeployment, produce a slice of Deployments,
    // where the ClusterName in each object's metadata MUST be provided.
}
```

## API Reference

https://godoc.org/admiralty.io/multicluster-scheduler/

or

```bash
go get admiralty.io/multicluster-scheduler
godoc -http=:6060
```

then http://localhost:6060/pkg/admiralty.io/multicluster-scheduler/
