# Multicluster-Scheduler

Multicluster-scheduler is a system of Kubernetes controllers that intelligently schedules workloads across clusters. It is simple to use and simple to integrate with other tools.

1. Install multicluster-scheduler in each cluster that you want to federate. Configure clusters as sources and/or targets to build a centralized or decentralized topology.
1. Annotate any pod or pod template (e.g., of a Deployment, Job, or [Argo](https://argoproj.github.io/projects/argo) Workflow, among others) in any source cluster with `multicluster.admiralty.io/elect=""`.
1. Multicluster-scheduler mutates the elected pods into _proxy pods_ scheduled on [virtual-kubelet](https://virtual-kubelet.io/) nodes representing target clusters, and creates _delegate pods_ in the remote clusters (actually running the containers).
1. A feedback loop updates the statuses and annotations of the proxy pods to reflect the statuses and annotations of the delegate pods.
1. Services that target proxy pods are rerouted to their delegates, replicated across clusters, and annotated with `io.cilium/global-service=true` to be [load-balanced across a Cilium cluster mesh](http://docs.cilium.io/en/stable/gettingstarted/clustermesh/#load-balancing-with-global-services), if installed (other integrations are possible, please [tell us about your network setup](#community)).

Check out [Admiralty's blog post](https://admiralty.io/blog/running-argo-workflows-across-multiple-kubernetes-clusters/) demonstrating how to run an Argo workflow across clusters to combine data from different regions or clouds and better utilize resources, or [ITNEXT's blog post](https://itnext.io/multicluster-scheduler-argo-workflows-across-kubernetes-clusters-ea98016499ca) describing an integration with [Argo CD](https://argoproj.github.io/projects/argo-cd) (scroll down to the relevant section). There are many other use cases: dynamic CDNs, multi-region high availability and disaster recovery, central access control and auditing, cloud bursting, clusters as cattle... [Tell us about your use case](#community).

## Getting Started

The first thing to understand is that there are two distinct kubernetes cluster types involved:

1. Source kubernetes clusters
1. Target kubernetes clusters

Each kubernetes cluster can play either one of the roles.  Or even both roles at the same time. In this document, we will however clearly mark source and target clusters for ease of understanding.

Admiralty has to be installed on both types of clusters for the federation to work.

Note that if a single person manages both types of clusters, that is great, but Admiralty can also be used to join clusters operated by several distinct administrative groups.

For this document we assume that you are a cluster admin for two clusters, associated with, e.g., the contexts "cluster1" and "cluster2" in your [kubeconfig](https://kubernetes.io/docs/tasks/access-application-cluster/configure-access-multiple-clusters/). We're going to install multicluster-scheduler in both clusters, and configure cluster1 as a source and target, and cluster2 as a target only. This topology is typical of a cloud bursting use case. Then, we will deploy a multi-cluster NGINX.


```bash
CLUSTER1=cluster1 # change me
CLUSTER2=cluster2 # change me
```

If you can only access one of the two clusters, just follow the intructions relevant to your cluster. In that case you can also remove the context part from all of the commands. Note that some parts need coordination between the admins of the two clusters; how messages are exchange in multi-admin setups is beyond the scope of this document.


Note: you can easily create two clusters on your machine with [kind](https://kind.sigs.k8s.io/). For larger, more realistic clusters you may want to consider using [kubeadm](https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/install-kubeadm/).

### Installation

#### Prerequisites

> **Important!** Multicluster-scheduler requires Kubernetes v1.17 or 1.18 (unless you build from source on a fork k8s.io/kubernetes, cf. [#19](https://github.com/admiraltyio/multicluster-scheduler/issues/19)).

Cert-manager v0.11+ must be installed in each cluster:

```sh
helm repo add jetstack https://charts.jetstack.io
helm repo update

for CONTEXT in $CLUSTER1 $CLUSTER2
do
  kubectl --context $CONTEXT apply --validate=false -f https://raw.githubusercontent.com/jetstack/cert-manager/release-0.12/deploy/manifests/00-crds.yaml
  kubectl --context $CONTEXT create namespace cert-manager
  helm install cert-manager \
    --kube-context $CONTEXT \
    --namespace cert-manager \
    --version v0.12.0 \
    --wait \
    jetstack/cert-manager
done
```

#### Optional: Cilium cluster mesh

For cross-cluster service calls, we rely in this guide on a Cilium cluster mesh and global services. If you need this feature, [install Cilium](http://docs.cilium.io/en/stable/gettingstarted/#installation) and [set up a cluster mesh](http://docs.cilium.io/en/stable/gettingstarted/clustermesh/). If you install Cilium later, you may have to restart pods.

#### Target Cluster install

The recommended way to install multicluster-scheduler on the target cluster is with Helm (v3):

```bash
helm repo add admiralty https://charts.admiralty.io
helm repo update

kubectl --context "$CLUSTER2" create namespace admiralty
helm install multicluster-scheduler admiralty/multicluster-scheduler \
  --kube-context "$CLUSTER2" \
  --namespace admiralty \
  --version 0.8.2 \
  --set clusterName=c2
```

Note that the target cluster does not need any information about the source cluster.

> **Important!** The target cluster must be accessible from the network of any source cluster you plan to federate with.

#### Source Cluster install

> **Important!** This section assumes you have not yet installed multicluster-scheduler on the source cluster. If you already have, and just want to add an additional target cluster, continue from the [Adding an additional Target Cluster to an existing Source Cluster section](#adding-an-additional-target-cluster-to-an-existing-source-cluster), instead.

While it is possible to install multicluster-scheduler on a source cluster without any known target clusters,
it is recommended you first decide which clusters it will target. In this example, we use the
target cluster installed above, named "c2".

The recommended way to install multicluster-scheduler on the source cluster is with Helm (v3):

```bash
helm repo add admiralty https://charts.admiralty.io
helm repo update

kubectl --context "$CLUSTER1" create namespace admiralty
helm install multicluster-scheduler admiralty/multicluster-scheduler \
  --kube-context "$CLUSTER1" \
  --namespace admiralty \
  --version 0.8.2 \
  --set clusterName=c1 \
  --set targetSelf=true \
  --set targets[0].name=c2
```

> **Important!** At this point, multicluster-scheduler will be stuck at ContainerCreating in cluster1, because it needs a secret from its remote target cluster2, see below. Note: when we move to defining targets at runtime with a CRD, this won't happen.

#### Service Account Exchange

For cross-cluster source-target communications, i.e., for multicluster-scheduler in a source cluster (here, cluster1) to talk to the Kubernetes API servers of remote target clusters (here, cluster2), we need to create service accounts in the target clusters, extract their tokens as kubeconfig files, and save those files inside secrets in their source clusters.

Note: for a source cluster that targets itself (here, cluster1), multicluster-scheduler simply uses its own service account to talk to its own Kubernetes API server.

In this getting started guide, we use [klum](https://github.com/ibuildthecloud/klum) to create a service account for cluster1 in cluster2 (there are other ways, [contact us](#community) while we work on documenting them).

In cluster2, install klum and create a User named `c1`, bound to the `multicluster-scheduler-source` cluster role at the cluster scope (you could bind it to one or several namespaces only, and configure multicluster-scheduler with namespaced targets, cf. [full installation guide](charts/multicluster-scheduler/README.md)).

```bash
kubectl --context "$CLUSTER2" apply -f https://raw.githubusercontent.com/ibuildthecloud/klum/v0.0.1/deploy.yaml

# klum registers the User CRD at runtime so wait a bit, then

cat <<EOF | kubectl --context "$CLUSTER2" apply -f -
kind: User
apiVersion: klum.cattle.io/v1alpha1
metadata:
  name: c1
spec:
  clusterRoles:
    - multicluster-scheduler-source
EOF
```

The `kubemcsa export` command of [multicluster-service-account](https://github.com/admiraltyio/multicluster-service-account#you-might-not-need-multicluster-service-account) makes it easy to prepare a kubeconfig secret. First, install kubemcsa (you don't need to deploy multicluster-service-account):

```bash
MCSA_RELEASE_URL=https://github.com/admiraltyio/multicluster-service-account/releases/download/v0.6.1
OS=linux # or darwin (i.e., OS X) or windows
ARCH=amd64 # if you're on a different platform, you must know how to build from source
curl -Lo kubemcsa "$MCSA_RELEASE_URL/kubemcsa-$OS-$ARCH"
chmod +x kubemcsa
```

Then, run `kubemcsa export` to generate a template for a secret containing a kubeconfig equivalent to the `c1` service account (that was created by klum), and apply the template with kubectl in cluster1:

```bash
./kubemcsa export --context "$CLUSTER2" -n klum c1 --as c2 \
  | kubectl --context "$CLUSTER1" -n admiralty apply -f -
```

Note: If you do not have access to both clusters, the admin of the target cluster (i.e. cluster2) can save the output of `kubemcsa export` into a file and deliver it to the admin of the source cluster (i.e. cluster1), who can then import it with `kubectl` from that file. 
Since the information in that file will contain secrets, the exchange should happen in a secure (e.g. encrypted) manner. What tools to use for that purpose is beyond the scope of this document (we're working on a convenient way to do that).

> **Important!** `kubemcsa export` combines a service account token with the Kubernetes API server addresses and associated certificates of the clusters found in your local kubeconfig. The addresses and certificates are routable and valid from your machine, but they need to be routable/valid from pods in the scheduler's cluster as well. For example, if you're using [kind](https://kind.sigs.k8s.io/), by default the address is `127.0.0.1:SOME_PORT`, because kind exposes API servers on random ports of your machine. However, `127.0.0.1` has a different meaning from the scheduler pod. On Linux, you can generate a kubeconfig with `kind get kubeconfig --internal` that will work from your machine and from pods, because it uses the master node container's IP in the overlay network (e.g., `172.17.0.x`), instead of `127.0.0.1`. Unfortunately, that won't work on Windows/Mac. In that case, you can either run the commands above from a container, or tweak the result of `kubemcsa export` before piping it into `kubectl apply`, to override the secret's `server` and `ca.crt` data fields (TODO: support overrides in `kubemcsa export`).

#### Verification on Source Cluster

After a minute, check that virtual nodes named `admiralty-c1` and `admiralty-c2` have been created in cluster1:

```bash
kubectl --context "$CLUSTER1" get node
```

### Multi-Cluster Deployment in Source Cluster

Multicluster-scheduler's pod admission controller operates in namespaces labeled with `multicluster-scheduler=enabled`. In cluster1, label the `default` namespace:

```bash
kubectl --context "$CLUSTER1" label namespace default multicluster-scheduler=enabled
```

Note: While we use the "default" namespace in this example, any other namespace could be used as well.
(but you will have to change the example accordingly, of course)

Then, deploy NGINX in it with the election annotation on the pod template:

```bash
cat <<EOF | kubectl --context "$CLUSTER1" apply -f -
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

1. The original pods have been transformed into proxy pods "running" on virtual nodes. Notice the original manifest saved as an annotation.
1. Delegate pods have been created in either cluster. Notice that their spec matches the original manifest.

```bash
kubectl --context "$CLUSTER1" get pods -o wide # (-o yaml for details)
kubectl --context "$CLUSTER2" get pods -o wide # (-o yaml for details)
```

Note: While launching and executing the pod requires only access to the source cluster (i.e. cluster1),
the above check requires access to both the source and target clusters. If you do not have direct access to the target cluster,
ask for help from someone who does.

### Advanced Scheduling

Multicluster-scheduler supports standard Kubernetes scheduling constraints including node selectors, affinities, etc. It will ensure that delegate pods target clusters that have nodes that match those constraints. For example, your nodes may be labeled with `failure-domain.beta.kubernetes.io/region` or `topology.kubernetes.io/region` (among other [common labels](https://kubernetes.io/docs/reference/kubernetes-api/labels-annotations-taints/)).

```sh
kubectl --context "$CLUSTER1" get nodes --show-labels
kubectl --context "$CLUSTER2" get nodes --show-labels
```

>If your test setup doesn't have region labels, you can add some:
>
>```sh
>kubectl --context "$CLUSTER1" label nodes -l virtual-kubelet.io/provider!=admiralty topology.kubernetes.io/region=us
>kubectl --context "$CLUSTER2" label nodes -l virtual-kubelet.io/provider!=admiralty topology.kubernetes.io/region=eu
>```

To schedule a deployment to a particular region, just add a node selector to its pod template:

```sh
kubectl --context "$CLUSTER1" patch deployment nginx -p '{
  "spec":{
    "template":{
      "spec": {
        "nodeSelector": {
          "topology.kubernetes.io/region": "eu"
        }
      }
    }
  }
}'
```

After a little while, delegate pods in cluster1 (US) will be terminated and more will be created in cluster2 (EU).

### Optional: Service Reroute and Globalization

Our NGINX deployment isn't much use without a service to expose it. [Kubernetes services](https://kubernetes.io/docs/concepts/services-networking/service/) route traffic to pods based on label selectors. We could directly create a service to match the labels of the delegate pods, but that would make it tightly coupled with multicluster-scheduler. Instead, let's create a service as usual, targeting the proxy pods. If a proxy pod were to receive traffic, it wouldn't know how to handle it, so multicluster-scheduler will change the service's label selector for us, to match the delegate pods instead, whose labels are similar to those of the proxy pods, except that their keys are prefixed with `multicluster.admiralty.io/`.

If some or all of the delegate pods are in a different cluster, we also need the service to route traffic to them. For that, we rely in this guide on a Cilium cluster mesh and global services. Multicluster-scheduler will annotate the service with `io.cilium/global-service=true` and replicate it across clusters. (Multicluster-scheduler replicates any global service across clusters, not just services targeting proxy pods.)

```bash
kubectl --context "$CLUSTER1" expose deployment nginx
```

We just created a service in cluster1, alongside our deployment. However, in the previous step, we rescheduled all NGINX pods to cluster2. Check that the service was rerouted, globalized, and replicated to cluster2:

```bash
kubectl --context "$CLUSTER1" get service nginx -o yaml
# Check the annotations and the selector,
# then check that a copy exists in cluster2:
kubectl --context "$CLUSTER2" get service nginx -o yaml
```

Now call the delegate pods in cluster2 from cluster1:

```bash
kubectl --context "$CLUSTER1" run foo -it --rm --image alpine --command -- sh -c "apk add curl && curl nginx"
```

#### Adding an additional Target Cluster to an existing Source Cluster

This section assumes you have already installed Admiralty on the source cluster (i.e. cluster1),
and just want to add an additional target cluster to it; we will keep the cluster2 name for consistency.

Assuming you installed multicluster-scheduler with Helm (v3), you must upgrade it with the same tool.

The easiest way is to retrieve the existing version of the configuration, and append the new cluster name to the targets section.

Note: You will need [jq](https://stedolan.github.io/jq/) for the command below to work.

```
helm get values multicluster-scheduler \
  --kube-context "$CLUSTER1" \
  --namespace admiralty \
  --output json | \
jq '.targets += [{name: "c2"}]' | \
helm upgrade multicluster-scheduler admiralty/multicluster-scheduler \
  --kube-context "$CLUSTER1" \
  --namespace admiralty \
  --version 0.8.2 \
  -f -
```

> **Important!** At this point, multicluster-scheduler will be stuck at ContainerCreating in cluster1, because it needs a secret from its remote target cluster2, see [Service Account Exchange section](#service-account-exchange) above. Note: when we move to defining targets at runtime with a CRD, this won't happen.

Continue with installation at the [Service Account Exchange section](#service-account-exchange) above.

## Community

Need help to install/use multicluster-scheduler or integrate it with your stack? Found a bug? Or perhaps you'd like to request or even contribute a feature. Please [file an issue](https://github.com/admiraltyio/multicluster-scheduler/issues/new/choose) or talk to us on [Admiralty's community chat](https://mattermost.admiralty.io).
