# Multicluster-Scheduler

Multicluster-scheduler is a system of Kubernetes controllers that intelligently schedules workloads across clusters. It is simple to use and simple to integrate with other tools.

1. Install the scheduler in any cluster and the agent in each cluster that you want to federate.
1. Annotate any pod or pod template (e.g., of a Deployment or [Argo](https://argoproj.github.io/argo) Workflow) in any member cluster with `multicluster.admiralty.io/elect=""`.
1. Multicluster-scheduler replaces the elected pods by proxy pods and deploys delegate pods to other clusters.
1. New in v0.3: Services that target proxy pods are rerouted to their delegates, replicated across clusters, and annotated with `io.cilium/global-service=true` to be [load-balanced across a Cilium cluster mesh](http://docs.cilium.io/en/stable/gettingstarted/clustermesh/#load-balancing-with-global-services), if installed.

Check out [Admiralty's blog post](https://admiralty.io/blog/running-argo-workflows-across-multiple-kubernetes-clusters/) demonstrating how to run an Argo workflow across clusters to better utilize resources and combine data from different regions or clouds.

## Getting Started

We assume that you are a cluster admin for two clusters, associated with, e.g., the contexts "cluster1" and "cluster2" in your kubeconfig. We're going to install a basic scheduler in cluster1 and agents in cluster1 and cluster2. Then, we will deploy a multi-cluster NGINX.

```bash
CLUSTER1=cluster1 # change me
CLUSTER2=cluster2 # change me
```

### Installation

#### Optional: Cilium cluster mesh

For cross-cluster service calls, multicluster-scheduler relies on a Cilium cluster mesh and global services. If you need this feature, [install Cilium](http://docs.cilium.io/en/stable/gettingstarted/#installation) and [set up a cluster mesh](http://docs.cilium.io/en/stable/gettingstarted/clustermesh/). If you install Cilium later, you may have to restart pods.

#### Helm

Starting from v0.4, the recommended way to install multicluster-scheduler is with Helm (v3):

```bash
helm repo add admiralty https://charts.admiralty.io
helm install multicluster-scheduler admiralty/multicluster-scheduler \
    --context $CLUSTER1 \
    -f test/e2e/single-namespace/values-cluster1.yaml
helm install multicluster-scheduler admiralty/multicluster-scheduler \
    --context $CLUSTER2 \
    -f test/e2e/single-namespace/values-cluster2.yaml
```

Note: the Helm chart is flexible enough to configure multiple federations and/or refine RBAC so clusters can't see each other's observations. While we work on properly documenting the chart, feel free reach out and/or check out the chart's own [values.yaml](charts/multicluster-scheduler/values.yaml), and some example `values.yaml` files under [test/e2e](test/e2e).

#### Service Account Exchange

For agents to talk to the scheduler across cluster boundaries (via custom resource definitions, cf. [How it Works](#how-it-works)), we need to export service accounts in the scheduler's cluster as kubeconfig files and save those files inside secrets in the agents' clusters.

Luckily, the `kubemcsa export` command of [multicluster-service-account](https://github.com/admiraltyio/multicluster-service-account#you-might-not-need-multicluster-service-account) can prepare the secrets for us. First, install kubemcsa (you don't need to deploy multicluster-service-account):

```bash
MCSA_RELEASE_URL=https://github.com/admiraltyio/multicluster-service-account/releases/download/v0.6.1
OS=linux # or darwin (i.e., OS X) or windows
ARCH=amd64 # if you're on a different platform, you must know how to build from source
curl -Lo kubemcsa "$MCSA_RELEASE_URL/kubemcsa-$OS-$ARCH"
chmod +x kubemcsa
```

Then, run `kubemcsa export` to generate templates for secrets containing kubeconfigs equivalent to the `c1` and `c2` service accounts created by Helm in cluster1, and apply the templates with kubectl in cluster1 and cluster2, respectively:

```bash
./kubemcsa export --context $CLUSTER1 c1 --as remote | kubectl --context $CLUSTER1 apply -f -
./kubemcsa export --context $CLUSTER1 c2 --as remote | kubectl --context $CLUSTER2 apply -f -
```

Note: you may wonder why the agent in cluster1 needs a kubeconfig as it runs in the same cluster as the scheduler. We simply like symmetry and didn't want to make the agent's configuration special in that case.

#### Verification

After a minute, check that node pool objects have been created in the agents' clusters and observations appear in the scheduler's cluster:

```bash
kubectl --context $CLUSTER1 get nodepools # or np
kubectl --context $CLUSTER2 get nodepools # or np

kubectl config use-context $CLUSTER1
kubectl get nodepoolobservations # or npobs
kubectl get nodeobservations # or nodeobs
kubectl get podobservations # or podobs
kubectl get serviceobservations # or svcobs
# or, by category
kubectl get observations --show-kind # or obs
```

### Example

Multicluster-scheduler's pod admission controller operates in namespaces labeled with `multicluster-scheduler=enabled`. In any of the member cluster, e.g., cluster2, label the `default` namespace:

```bash
kubectl --context "$CLUSTER2" label namespace default multicluster-scheduler=enabled
```

Then, deploy NGINX in it with the election annotation on the pod template:

```bash
cat <<EOF | kubectl --context "$CLUSTER2" apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
spec:
  replicas: 10
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
      annotations:
        multicluster.admiralty.io/elect: ""
    spec:
      containers:
      - name: nginx
        image: nginx
        resources:
          requests:
            cpu: 100m
            memory: 32Mi
        ports:
        - containerPort: 80
EOF
```

Things to check:

1. The original pods have been transformed into proxy pods. Notice the replacement containers, and the original manifest saved as an annotation.
1. Proxy pod observations have been created in the scheduler's cluster.
1. Delegate pod decisions have been created in the scheduler's cluster as well. Each decision was made based on all of the observations available at the time.
1. Delegate pods have been created in either cluster. Notice that their spec matches the original manifest.

```bash
kubectl --context "$CLUSTER2" get pods # (-o yaml for details)
kubectl --context "$CLUSTER1" get podobs # (-o yaml)
kubectl --context "$CLUSTER1" get poddecisions # or poddec (-o yaml)

kubectl --context "$CLUSTER1" get pods # (-o yaml)
kubectl --context "$CLUSTER2" get pods # (-o yaml)
```

### Enforcing Placement

In some cases, you may want to specify the target cluster, rather than let the scheduler decide. For example, you may want an Argo workflow to execute certain steps in certain clusters, e.g., to be closer to external dependencies. You can enforce placement using the `multicluster.admiralty.io/clustername` annotation. [Admiralty's blog post](https://admiralty.io/blog/using-admiralty-s-multicluster-scheduler-to-run-argo-workflows-across-kubernetes-clusters) presents multicloud Argo workflows. To complete this getting started guide, let's annotate our NGINX deployment's pod template to reschedule all pods to cluster1.

```bash
kubectl --context "$CLUSTER2" patch deployment nginx -p '{
  "spec":{
    "template":{
      "metadata":{
        "annotations":{
          "multicluster.admiralty.io/clustername":"c1"
        }
      }
    }
  }
}'
```

After a little while, delegate pods in cluster2 will be terminated and more will be created in cluster1.

### Optional: Service Reroute and Globalization

Our NGINX deployment isn't much use without a service to expose it. [Kubernetes services](https://kubernetes.io/docs/concepts/services-networking/service/) route traffic to pods based on label selectors. We could directly create a service to match the labels of the delegate pods, but that would make it tightly coupled with multicluster-scheduler. Instead, let's create a service as usual, targeting the proxy pods. If a proxy pod were to receive traffic, it wouldn't know how to handle it, so multicluster-scheduler will change the service's label selector for us, to match the delegate pods instead, whose labels are similar to those of the proxy pods, except that their keys are prefixed with `multicluster.admiralty.io/`.

If some or all of the delegate pods are in a different cluster, we also need the service to route traffic to them. For that, we rely on a Cilium cluster mesh and global services. Multicluster-scheduler will annotate the service with `io.cilium/global-service=true` and replicate it across clusters. (Multicluster-scheduler replicates any global service across clusters, not just services targeting proxy pods.)

```bash
kubectl --context "$CLUSTER2" expose deployment nginx
```

We just created a service in cluster2, alongside our deployment. However, in the previous step, we rescheduled all NGINX pods to cluster1. Check that the service was rerouted, globalized, and replicated to cluster1:

```bash
kubectl --context "$CLUSTER2" get service nginx -o yaml
# Check the annotations and the selector,
# then check that a copy exists in cluster1:
kubectl --context "$CLUSTER1" get service nginx -o yaml
```

Now call the delegate pods in cluster1 from cluster2:

```bash
kubectl --context "$CLUSTER2" run foo -it --rm --image alpine --command -- sh -c "apk add curl && curl nginx"
```

## How it Works

![](docs/multicluster-scheduler-sequence-diagram.svg)

Multicluster-scheduler is a system of Kubernetes controllers managed by the **scheduler**, deployed in any cluster, and its **agents**, deployed in the member clusters. The scheduler manages two controllers: the eponymous scheduler controller and the global service controller. Each agent manages six controllers: pod admission, service reroute, observations, decisions, feedback, and node pool.

1. The **pod admission controller**, a [dynamic, mutating admission webhook](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/), intercepts pod creation requests. If a pod is annotated with `multicluster.admiralty.io/elect=""`, its original manifest is saved as an annotation, and its containers are replaced by **proxies** that simply await success or failure signals from the feedback controller (see below).
1. The **service reroute controller** modifies services whose endpoints target proxy pods. The keys of their label selectors are prefixed with `multicluster.admiralty.io/`, to match corresponding **delegate** pods (see below). Also, the services are annotated with `io.cilium/global-service=true`, to be load-balanced across a Cilium cluster mesh.
1. The **observations controller**, a [multi-cluster controller](https://github.com/admiraltyio/multicluster-controller), watches pods (including proxy pods), services (including global services), nodes, and node pools (created by the node pool controller, see below) in the agent's cluster and reconciles corresponding **observations** in the scheduler's cluster. Observations are images of the source objects' states.
1. The **scheduler** watches proxy pod observations and reconciles delegate pod **decisions**, all in its own cluster. It decides target clusters based on the other observations. The scheduler doesn't push anything to the member clusters.
1. The **global service controller** watches global service observations (observations of services annotated with `io.cilium/global-service=true`, either by the service reroute controller or by another tool or user) and reconciles global service decisions (copies of the originals), in all clusters of the federation.
1. The **decisions controller**, another multi-cluster controller, watches pod and service decisions in the scheduler's cluster and reconciles corresponding delegates in the agent's cluster.
1. The **feedback controller** watches delegate pod observations and reconciles the corresponding proxy pods. If a delegate pod is annotated (e.g., with Argo outputs), the annotations are replicated upstream, into the corresponding proxy pod. When a delegate pod succeeds or fails, the controller kills the corresponding proxy pod's container with the proper signal. The feedback controller maintains the contract between proxy pods and their controllers, e.g., replica sets or Argo workflows.
1. The **node pool controller** automatically creates **node pool** objects in the agent's cluster. In GKE and AKS, it uses the `cloud.google.com/gke-nodepool` or `agentpool` label, respectively; in the absence of those labels, a default node pool object is created. Min/max node counts and pricing information can be updated by the user, or controlled by other tools. Custom node pool objects can also be created using label selectors. Node pool information can be used for scheduling.

Observations, decisions, and node pools are [custom resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/). Node pools are defined (by CRDs) in each member cluster, whereas all observations and decisions are only defined in the scheduler's cluster.

## Comparison with Kubefed (Federation v2)

The goal of [Kubefed](https://github.com/kubernetes-sigs/kubefed) is similar to multicluster-scheduler's. However, they differ in several ways:

- Kubefed has a broader scope than multicluster-scheduler. Any resource can be federated and deployed via a single control plane, even if the same result could be achieved with continuous delivery, e.g., GitOps. Multicluster-scheduler focuses on _scheduling_.
- Multicluster-scheduler doesn't require using new federated resource types (cf. Kubefed's templates, placements and overrides). Instead, pods only need to be annotated to be scheduled to other clusters. This makes adopting multicluster-scheduler painless and ensures compatibility with other tools like Argo.
- Whereas Kubefed's API resides in a single cluster, multicluster-scheduler's annotated pods can be declared in any member cluster and/or the scheduler's cluster. Teams can keep working in separate clusters, while utilizing available resources in other clusters as needed. 
- Kubefed propagates scheduling resources with a push-sync reconciler. Multicluster-scheduler's agents push observations and pull scheduling decisions to/from the scheduler's cluster. The scheduler reconciles scheduling decisions with observations, but never calls the Kubernetes APIs of the member clusters. Clusters allowing outbound traffic to, but no inbound traffic from the scheduler's cluster (e.g., on-prem, in some cases) can join the federation. Also, if the scheduler's cluster is compromised, attackers don't automatically gain access to the entire federation.
- Kubefed integrates with [ExternalDNS](https://github.com/kubernetes-incubator/external-dns) to provide [cross-cluster service discovery](https://github.com/kubernetes-sigs/federation-v2/blob/master/docs/servicedns-with-externaldns.md) and [multicluster ingress](https://github.com/kubernetes-sigs/federation-v2/blob/master/docs/ingressdns-with-externaldns.md). Multicluster-scheduler doesn't solve multicluster ingress at the moment, but integrates with Cilium for cross-cluster service discovery, and [everything else Cilium has to offer](https://cilium.readthedocs.io/en/latest/intro/). A detailed comparison of the two approaches is beyond the scope of this README (but certainly worth a future blog post).

## Roadmap

- [x] [Integration with Argo](https://admiralty.io/blog/running-argo-workflows-across-multiple-kubernetes-clusters/)
- [x] Integration with Cilium cluster mesh and global services
- [x] One namespace per member cluster in the scheduler's cluster for more granular RBAC
- [ ] Alternative cross-cluster networking implementations: Istio (1.1), Submariner
- [ ] More integrations: Horizontal Pod Autoscaler, Knative, Rook, k3s, kube-batch
- [ ] Advanced scheduling, respecting affinities, anti-affinities, taints, tolerations, quotas, etc.
- [ ] Port-forward between proxy and delegate pods
- [ ] Integrate node pool concept with other tools

## API Reference

https://godoc.org/admiralty.io/multicluster-scheduler/

or

```bash
go get admiralty.io/multicluster-scheduler
godoc -http=:6060
```

then http://localhost:6060/pkg/admiralty.io/multicluster-scheduler/
