# Multicluster-Scheduler

Multicluster-scheduler is a system of Kubernetes controllers that intelligently schedules workloads across clusters. It is simple to use and simple to integrate with other tools.

1. Install the scheduler in any cluster and the agent in each cluster that you want to federate.
1. Annotate any pod or pod template (e.g., of a Deployment, Job, or [Argo](https://argoproj.github.io/argo) Workflow, among others) in any member cluster with `multicluster.admiralty.io/elect=""`.
1. Multicluster-scheduler mutates the elected pods into proxy pods scheduled on a [virtual-kubelet](https://virtual-kubelet.io/) node, and creates delegate pods in remote clusters (actually running the containers).
1. A feedback loop updates the statuses and annotations of the proxy pods to reflect the statuses and annotations of the delegate pods.
1. Services that target proxy pods are rerouted to their delegates, replicated across clusters, and annotated with `io.cilium/global-service=true` to be [load-balanced across a Cilium cluster mesh](http://docs.cilium.io/en/stable/gettingstarted/clustermesh/#load-balancing-with-global-services), if installed.

Check out [Admiralty's blog post](https://admiralty.io/blog/running-argo-workflows-across-multiple-kubernetes-clusters/) demonstrating how to run an Argo workflow across clusters to combine data from different regions or clouds and better utilize resources. There are many other use cases. Tell us about yours!

## Getting Started

We assume that you are a cluster admin for two clusters, associated with, e.g., the contexts "cluster1" and "cluster2" in your kubeconfig. We're going to install a basic scheduler in cluster1 and agents in cluster1 and cluster2. Then, we will deploy a multi-cluster NGINX.

```bash
CLUSTER1=cluster1 # change me
CLUSTER2=cluster2 # change me
```

Note: you can easily create two clusters on your machine with [kind](https://kind.sigs.k8s.io/).

### Installation

#### Prerequisites

Cert-manager v0.11+ must be installed in each member cluster:

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
    jetstack/cert-manager
done
```

#### Optional: Cilium cluster mesh

For cross-cluster service calls, multicluster-scheduler relies on a Cilium cluster mesh and global services. If you need this feature, [install Cilium](http://docs.cilium.io/en/stable/gettingstarted/#installation) and [set up a cluster mesh](http://docs.cilium.io/en/stable/gettingstarted/clustermesh/). If you install Cilium later, you may have to restart pods.

#### Helm

The recommended way to install multicluster-scheduler is with Helm (v3):

```bash
helm repo add admiralty https://charts.admiralty.io
helm repo update

kubectl --context $CLUSTER1 create namespace admiralty
helm install multicluster-scheduler admiralty/multicluster-scheduler \
  --kube-context $CLUSTER1 \
  --namespace admiralty \
  --version 0.7.0 \
  --set scheduler.enabled=true \
  --set scheduler.clusters[0].name=c1 \
  --set scheduler.clusters[1].name=c2 \
  --set agent.enabled=true \
  --set agent.invitations[0].clusterName=c1 \
  --set agent.invitations[1].clusterName=c2

kubectl --context $CLUSTER2 create namespace admiralty
helm install multicluster-scheduler admiralty/multicluster-scheduler \
  --kube-context $CLUSTER2 \
  --namespace admiralty \
  --version 0.7.0 \
  --set agent.enabled=true \
  --set agent.invitations[0].clusterName=c1 \
  --set agent.invitations[1].clusterName=c2
```

#### Service Account Exchange

For the scheduler to talk to the member clusters' Kubernetes API servers, we need to extract service account tokens from the member clusters as kubeconfig files, and save those files inside secrets in the scheduler's cluster.

Luckily, the `kubemcsa export` command of [multicluster-service-account](https://github.com/admiraltyio/multicluster-service-account#you-might-not-need-multicluster-service-account) can prepare the secrets for us. First, install kubemcsa (you don't need to deploy multicluster-service-account):

```bash
MCSA_RELEASE_URL=https://github.com/admiraltyio/multicluster-service-account/releases/download/v0.6.1
OS=linux # or darwin (i.e., OS X) or windows
ARCH=amd64 # if you're on a different platform, you must know how to build from source
curl -Lo kubemcsa "$MCSA_RELEASE_URL/kubemcsa-$OS-$ARCH"
chmod +x kubemcsa
```

Then, for each member cluster, run `kubemcsa export` to generate a template for a secret containing a kubeconfig equivalent to the service account named `multicluster-scheduler-agent-for-scheduler` (that was created by Helm), and apply the template with kubectl in the scheduler's cluster:

```bash
./kubemcsa export --context $CLUSTER1 -n admiralty multicluster-scheduler-agent-for-scheduler --as c1 \
  | kubectl --context $CLUSTER1 -n admiralty apply -f -
./kubemcsa export --context $CLUSTER2 -n admiralty multicluster-scheduler-agent-for-scheduler --as c2 \
  | kubectl --context $CLUSTER1 -n admiralty apply -f -
```

> Note: You may wonder why the scheduler needs a kubeconfig for cluster1, which is were it runs. We simply like symmetry and didn't want to make the configuration special in that case (when the scheduler's cluster is also a member cluster).

> **Important!** `kubemcsa export` combines a service account token with the Kubernetes API server addresses and associated certificates of the member clusters found in your local kubeconfig. The addresses and certificates are routable and valid from your machine, but they need to be routable/valid from pods in the scheduler's cluster as well. For example, if you're using [kind](https://kind.sigs.k8s.io/), by default the address is `127.0.0.1:SOME_PORT`, because kind exposes API servers on random ports of your machine. However, `127.0.0.1` has a different meaning from the scheduler pod. On Linux, you can generate a kubeconfig with `kind get kubeconfig --internal` that will work from your machine and from pods, because it uses the master node container's IP in the overlay network (e.g., `172.17.0.x`), instead of `127.0.0.1`. Unfortunately, that won't work on Windows/Mac. In that case, you can either run the commands above from a container, or tweak the result of `kubemcsa export` before piping it into `kubectl apply`, to override the secret's `server` and `ca.crt` data fields (TODO: support overrides in `kubemcsa export`).

#### Verification

After a minute, check that a virtual node named `admiralty` and node pool objects have been created in each cluster:

```bash
kubectl --context $CLUSTER1 get node
kubectl --context $CLUSTER2 get node

kubectl --context $CLUSTER1 get nodepools # or np
kubectl --context $CLUSTER2 get nodepools # or np
```

### Multi-Cluster Deployment

Multicluster-scheduler's pod admission controller operates in namespaces labeled with `multicluster-scheduler=enabled`. In any of the member clusters, e.g., cluster2, label the `default` namespace:

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

1. The original pods have been transformed into proxy pods "running" on virtual nodes. Notice the original manifest saved as an annotation.
1. Delegate pods have been created in either cluster. Notice that their spec matches the original manifest.

```bash
kubectl --context "$CLUSTER2" get pods -o wide # (-o yaml for details)
kubectl --context "$CLUSTER1" get pods -o wide # (-o yaml for details)
```

### Node Selector

Multicluster-scheduler supports standard Kubernetes node selectors as a scheduling constraint. It will ensure that delegate pods target clusters that have nodes that match the selector. For example, your nodes may be labeled with `failure-domain.beta.kubernetes.io/region` or `topology.kubernetes.io/region` (among other [common labels](https://kubernetes.io/docs/reference/kubernetes-api/labels-annotations-taints/)).

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
kubectl --context "$CLUSTER2" patch deployment nginx -p '{
  "spec":{
    "template":{
      "spec": {
        "nodeSelector": {
          "topology.kubernetes.io/region": "eu" # change me
        }
      }
    }
  }
}'
```

After a little while, delegate pods in cluster1 (US) will be terminated and more will be created in cluster2 (EU).

### Cross-Cluster RBAC

While installing multicluster-scheduler, you may have noticed that we configured _invitations_. Invitations create (cluster) role bindings between users named after the invited clusters, e.g., `admiralty:c1`, and the `multicluster-scheduler-agent-for-cluster` cluster role. The scheduler impersonates those users when creating/updating/deleting delegate pods and services on behalf of invited clusters,]. Beforehand, it uses the `SelfSubjectAccessReview` API (like `kubectl auth can-i`) to filter clusters.

If an invitation doesn't specify namespaces, it is valid for all namespaces.

Delete the existing deployment:

```sh
kubectl --context "$CLUSTER2" delete deployment nginx
```

Update your installation to only invite cluster2 in cluster1's `c1-for-c2` namespace, and remove all invitations in cluster2 (e.g., to represent a control-plane-only cluster):

```sh
kubectl --context $CLUSTER1 create namespace c1-for-c2
helm upgrade --reuse-values multicluster-scheduler admiralty/multicluster-scheduler \
  --kube-context $CLUSTER1 \
  --namespace admiralty \
  --version 0.7.0 \
  --set agent.invitations[1].namespaces[0]=c1-for-c2

kubectl --context $CLUSTER2 create namespace c1-for-c2
helm upgrade --reuse-values multicluster-scheduler admiralty/multicluster-scheduler \
  --kube-context $CLUSTER2 \
  --namespace admiralty \
  --version 0.7.0 \
  --set agent.invitations=
```

Now, try to create the same multi-cluster deployment as above in the `default` namespace of cluster2. It will stay pending as there are no available clusters for it.

Them, create the same multi-cluster deployment in the `c1-for-c2` namespace of cluster2. Pods will be created in cluster1.

### Enforcing Placement

In some cases, you may want to specify a target cluster, rather than let the scheduler decide. You can enforce placement using the `multicluster.admiralty.io/clustername` annotation. To complete this getting started guide, let's annotate our NGINX deployment's pod template to reschedule all pods to cluster1.

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
