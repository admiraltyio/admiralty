# Multicluster-Scheduler

Multicluster-scheduler is a system of Kubernetes controllers that intelligently schedules workloads across clusters. It is simple to use and simple to integrate with other tools.

1. Install multicluster-scheduler in each cluster that you want to federate. Configure clusters as sources and/or targets to build a centralized or decentralized topology.
1. Annotate any pod or pod template (e.g., of a Deployment, Job, or [Argo](https://argoproj.github.io/projects/argo) Workflow, among others) in any source cluster with `multicluster.admiralty.io/elect=""`.
1. Multicluster-scheduler mutates the elected pods into _proxy pods_ scheduled on [virtual-kubelet](https://virtual-kubelet.io/) nodes representing target clusters, and creates _delegate pods_ in the remote clusters (actually running the containers).
1. A feedback loop updates the statuses and annotations of the proxy pods to reflect the statuses and annotations of the delegate pods.
1. Services that target proxy pods are rerouted to their delegates, replicated across clusters, and annotated with `io.cilium/global-service=true` to be [load-balanced across a Cilium cluster mesh](http://docs.cilium.io/en/stable/gettingstarted/clustermesh/#load-balancing-with-global-services), if installed. (Other integrations are possible, e.g., with [Linkerd](https://linkerd.io/2/features/multicluster/) or [Istio](https://istio.io/latest/docs/ops/deployment/deployment-models/#multiple-clusters); please [tell us about your network setup](#community).)

Check out [Admiralty's blog post](https://admiralty.io/blog/running-argo-workflows-across-multiple-kubernetes-clusters/) demonstrating how to run an Argo workflow across clusters to combine data from different regions or clouds and better utilize resources, or [ITNEXT's blog post](https://itnext.io/multicluster-scheduler-argo-workflows-across-kubernetes-clusters-ea98016499ca) describing an integration with [Argo CD](https://argoproj.github.io/projects/argo-cd) (scroll down to the relevant section).

There are many other use cases: dynamic CDNs, multi-region high availability and disaster recovery, central access control and auditing, cloud bursting, clusters as cattle, blue/green cluster upgrades, edge computing, Internet of Things (IoT)... [Tell us about your use case and/or help us write specific usage guides](#community).

## Getting Started

The first thing to understand is that clusters can be **either or both** sources and/or targets. Multicluster-scheduler has to be installed in all clusters.

In this guide, we assume that you are a cluster admin for two clusters, associated with, e.g., the contexts "cluster1" and "cluster2" in your [kubeconfig](https://kubernetes.io/docs/tasks/access-application-cluster/configure-access-multiple-clusters/). We're going to install multicluster-scheduler in both clusters, and configure cluster1 as a source and target, and cluster2 as a target only. This topology is typical of a cloud bursting use case. Then, we will deploy a multi-cluster NGINX.

```bash
CLUSTER1=cluster1 # change me
CLUSTER2=cluster2 # change me
```

⚠️ The Kubernetes API server address of a target cluster must be routable from pods running multicluster-scheduler in its source clusters. Here, pods in cluster1 can obviously reach their own cluster's Kubernetes API server. They must also be able to reach cluster2's Kubernetes API server.
- With cloud distributions (e.g., EKS, AKS, GKE, etc.), by default, the Kubernetes API server is often exposed publicly (but securely) on the Internet, while pods can call out freely to the Internet, so you should be good to go.
- For private clusters, you might have to set up a VPN or tunnel (we have plans to help with that).
- While experimenting with [kind](https://kind.sigs.k8s.io/), [minikube](https://minikube.sigs.k8s.io/) or [k3d](https://k3d.io/) clusters, where Kubernetes nodes run as containers (on Linux) or VMs (on Mac/Windows) on your machine, you may have to edit the kubeconfig given to cluster1 to access cluster2 (cf. [Service Account Exchange](#2-service-account-exchange) step, below). The Kubernetes API server is usually exposed on your machine's loopback IP (127.0.0.1), which is what you use from your machine, but means something different from pods—i.e., their own loopback interface. You'll need to replace the address with either your machine's address from containers/VMs (though the CA certificate may not match in that case), or cluster2's master node address on the Docker network shared by the two clusters (assuming they share a Docker network).

<details>
  <summary>ℹ️ If you can only access one of the two clusters, ...</summary>

  just follow the instructions relevant to your cluster. If a single person manages all clusters, that's great, but multicluster-scheduler can also be used to join clusters operated by several distinct administrative groups. In that case you can also remove the context part from all of the commands. Note that some parts need coordination between the admins of the two clusters; how messages are exchanged in multi-admin setups is beyond the scope of this document.

</details>

### Prerequisites

⚠️ Multicluster-scheduler requires Kubernetes v1.17 or 1.18 (unless you build from source on a fork k8s.io/kubernetes, cf. [#19](https://github.com/admiraltyio/multicluster-scheduler/issues/19)).

Cert-manager v0.11+ must be installed in each cluster:

```sh
helm repo add jetstack https://charts.jetstack.io
helm repo update

for CONTEXT in $CLUSTER1 $CLUSTER2
do
  kubectl --context $CONTEXT apply --validate=false -f https://raw.githubusercontent.com/jetstack/cert-manager/release-0.12/deploy/manifests/00-crds.yaml
  kubectl --context $CONTEXT create namespace cert-manager
  helm install cert-manager jetstack/cert-manager \
    --kube-context $CONTEXT \
    --namespace cert-manager \
    --version v0.12.0 \
    --wait
done
```

### Optional: Cilium cluster mesh

For cross-cluster service calls, we rely in this guide on a Cilium cluster mesh and global services. If you need this feature, [install Cilium](http://docs.cilium.io/en/stable/gettingstarted/#installation) and [set up a cluster mesh](http://docs.cilium.io/en/stable/gettingstarted/clustermesh/). If you install Cilium later, you may have to restart pods.

### Installation

The recommended way to install multicluster-scheduler is with Helm (v3):

```bash
helm repo add admiralty https://charts.admiralty.io
helm repo update

for CONTEXT in $CLUSTER1 $CLUSTER2
do
  kubectl --context "$CONTEXT" create namespace admiralty
  helm install multicluster-scheduler admiralty/multicluster-scheduler \
  --kube-context "$CONTEXT" \
  --namespace admiralty \
  --version 0.10.0-rc.1 \
  --wait
done
```

### Connecting Source and Target Clusters

At this point, multicluster-scheduler is installed in both clusters, but neither is a source or target yet.

1. Target clusters need service accounts and RBAC rules to autenticate and authorize source clusters, cf. [Creating Service Accounts and RBAC Rules in Target Clusters](#1-creating-service-accounts-and-rbac-rules-in-target-clusters), below.

2. Source clusters need those service account's tokens, and their targets' server addresses and CA certificates, saved as kubeconfig files in secrets, cf. [Service Account Exchange](#2-service-account-exchange), below.

3. You also need to tell multicluster-scheduler in source clusters which kubeconfig secret corresponds to which target, and if targets are namespaced or cluster-scoped, using Target and ClusterTarget custom resources objects, respectively, cf. [Creating ClusterTargets and Targets in Source Clusters](#3-creating-clustertargets-and-targets-in-source-clusters), below.

 ⚠️ For a source cluster that targets itself (here, cluster1), multicluster-scheduler simply uses its own service account to talk to its own Kubernetes API server. For that connection, you only need to create a ClusterTarget or namespaced Target with `spec.self=true`.

<details>
  <summary>ℹ️ If you're concerned about the spread of service account tokens, ...</summary>

  there are other ways to solve cross-cluster authentication, including the use of cloud service accounts, also known as "service principals" or "roles", as opposed to Kubernetes service accounts; or public key infrastructure (PKI) solutions. [Contact us](#community) while we work on documenting them.

</details>

#### 1. Creating Service Accounts and RBAC Rules in Target Clusters

In this guide, we use [klum](https://github.com/ibuildthecloud/klum) to create a service account and RBAC rules for cluster1 in cluster2. In cluster2, install klum and create a User named `c1`, bound to the `multicluster-scheduler-source` and `multicluster-scheduler-cluster-summary-viewer` cluster roles at the cluster scope.

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
    - multicluster-scheduler-cluster-summary-viewer
EOF
```

<details>
  <summary>ℹ If you want to allow a source in specific namespaces only, ...</summary>
  
  you can bind `multicluster-scheduler-source` to namespaces, and configure multicluster-scheduler with namespaced Targets (see below), but `multicluster-scheduler-cluster-summary-viewer` _must_ be cluster-bound, because clustersummaries is a cluster-scoped custom resource. Though have no fear: all `multicluster-scheduler-cluster-summary-viewer` allows is get/list/watch ClusterSummaries, which are cluster singletons that only contain the sum of the capacities and allocatable resources of their respective clusters' nodes.

</details>

<details>
  <summary>ℹ If you don't want to use klum, ...</summary>
  
  you can create a ServiceAccount and ClusterRoleBindings and/or RoleBindings directly, e.g.:
  
  ```bash
  cat <<EOF | kubectl --context "$CLUSTER2" apply -f -
  apiVersion: v1
  kind: ServiceAccount
  metadata:
    name: c1
    namespace: admiralty
  ---
  apiVersion: rbac.authorization.k8s.io/v1
  kind: ClusterRoleBinding
  metadata:
    name: c1-multicluster-scheduler-source
  roleRef:
    apiGroup: rbac.authorization.k8s.io
    kind: ClusterRole
    name: multicluster-scheduler-source
  subjects:
    - kind: ServiceAccount
      name: c1
      namespace: admiralty
  ---
  apiVersion: rbac.authorization.k8s.io/v1
  kind: ClusterRoleBinding
  metadata:
    name: c1-multicluster-scheduler-cluster-summary-viewer
  roleRef:
    apiGroup: rbac.authorization.k8s.io
    kind: ClusterRole
    name: multicluster-scheduler-cluster-summary-viewer
  subjects:
    - kind: ServiceAccount
      name: c1
      namespace: admiralty
  EOF
  ```

</details>

#### 2. Service Account Exchange

At this point, cluster1 has an identity on cluster2 and is authorized to do its job. It still needs the address of cluster2's Kubernetes API server, the corresponding CA certificate to authenticate the server, and its new identity's credentials (here, a service account token) for mutual authentication by the server. This information can be stored in a standard kubeconfig file saved in a secret in cluster1.

The `kubemcsa export` command of [multicluster-service-account](https://github.com/admiraltyio/multicluster-service-account#you-might-not-need-multicluster-service-account) makes it easy to prepare a kubeconfig secret. First, install kubemcsa (you don't need to deploy multicluster-service-account):

```bash
MCSA_RELEASE_URL=https://github.com/admiraltyio/multicluster-service-account/releases/download/v0.6.1
OS=linux # or darwin (i.e., OS X) or windows
ARCH=amd64 # if you're on a different platform, you must know how to build from source
curl -Lo kubemcsa "$MCSA_RELEASE_URL/kubemcsa-$OS-$ARCH"
chmod +x kubemcsa
```

Then, run `kubemcsa export` to generate a template for a secret containing a kubeconfig equivalent to the `c1` service account (that was created by klum or yourself, above), and apply the template with kubectl in cluster1:

```bash
./kubemcsa export --context "$CLUSTER2" -n klum c1 --as c2 \
  | kubectl --context "$CLUSTER1" -n admiralty apply -f -
```

⚠️ `kubemcsa export` combines a service account token with the Kubernetes API server address and associated CA certificate of the cluster found in your local kubeconfig. The address and CA certificate are routable and valid from your machine, but they need to be routable/valid from pods in the source cluster as well. For example, if you're using [kind](https://kind.sigs.k8s.io/), by default the address is `127.0.0.1:SOME_PORT`, because kind exposes API servers on random ports of your machine. However, `127.0.0.1` has a different meaning from a multicluster-scheduler pod. On Linux, you can generate a kubeconfig with `kind get kubeconfig --internal` that will work from your machine and from pods, because it uses the master node container's IP in the overlay network (e.g., `172.17.0.x`), instead of `127.0.0.1`. Unfortunately, that won't work on Windows/Mac. In that case, you can either run the commands above from a container, or tweak the result of `kubemcsa export` before piping it into `kubectl apply`, to override the secret's `server` and `ca.crt` data fields (TODO: support overrides in `kubemcsa export` and provide detailed instructions on different platforms).

<details>
  <summary>ℹ If you don't have access to both clusters, ...</summary>

  the admin of the target cluster (i.e. cluster2) can save the output of `kubemcsa export` into a file and deliver it to the admin of the source cluster (i.e. cluster1), who can then import it with `kubectl` from that file. Since the information in that file will contain secrets, the exchange should happen in a secure (e.g. encrypted) manner. What tools to use for that purpose is beyond the scope of this document (we're working on a convenient way to do that).

</details>

<details>
  <summary>ℹ If you don't want to use kubemcsa, ...</summary>
  
  here are equivalent commands using kubectl and the more ubiquitous [jq](https://stedolan.github.io/jq/):
  
  ```bash
  SECRET_NAME=$(kubectl --context "$CLUSTER2" get serviceaccount c1 \
    --namespace klum \
    --output json | \
    jq --raw .secrets[0].name)
  
  TOKEN=$(kubectl --context "$CLUSTER2" get secret $SECRET_NAME \
    --namespace klum \
    --output json | \
    jq --raw .data.token | \
    base64 --decode)
  
  CONFIG=$(kubectl --context "$CLUSTER2" config view \
    --minify \
    --raw \
    --output json | \
    jq '.users[0].user={token:"'$TOKEN'"} | \
      .contexts[0].context.namespace="default"')

  kubectl --context "$CLUSTER1" create secret generic c2 \
    --from-literal=config="$CONFIG"
  ```

  In a multi-admin scenario, replace the last command with this one to produce the secret file to send to your peer, who will `kubectl apply` it:

  ```
  kubectl create secret generic c2 \
      --from-literal=config="$KUBECONFIG" \
      --dry-run \
      --output yaml > kubeconfig-secret.yaml
  ```
</details>

#### 3. Creating ClusterTargets and Targets in Source Clusters

At this point, cluster1 has an identity in cluster2, credentials to be authenticated by cluster2, knows where to find and authenticate cluster2. You just need to tell multicluster-scheduler where to find and how to use that information.

ClusterTargets and Targets are Kubernetes [custom resources](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) installed with multicluster-scheduler:

- A cluster-scoped ClusterTarget associates a name (`metadata.name`) with a reference to a kubeconfig secret (name and namespace; can be in any namespace but it makes sense to store these in the same namespace as multicluster-scheduler, e.g., `admiralty`, as we did). Multicluster-scheduler will use the kubeconfig to interact with resources in the target cluster at the cluster level, so it must be authorized to do so there (i.e., there must be a ClusterRoleBinding between the `multicluster-scheduler-source` ClusterRole and cluster1's identity).

- A namespaced Target associates a name with a reference to a kubeconfig secret (name only, in the same namespace). Multicluster-scheduler will use the kubeconfig to interact with resources in the target cluster in the eponymous namespace (except to view the cluster-scoped ClusterSummary, as explained above), so it must be authorized to do so there (i.e., there must be a RoleBinding between the `multicluster-scheduler-source` ClusterRole and cluster1's identity in that namespace).

ClusterTargets and Targets can also target their local cluster, e.g., for cloud bursting. In that case, they don't reference a kubeconfig secret, but specify `spec.self=true` instead. In our case, apply the following two ClusterTargets in cluster1:

```bash
cat <<EOF | kubectl --context "$CLUSTER1" apply -f -
kind: ClusterTarget
apiVersion: multicluster.admiralty.io/v1alpha1
metadata:
  name: c2
spec:
  kubeconfigSecret:
    name: c2
    namespace: admiralty
---
kind: ClusterTarget
apiVersion: multicluster.admiralty.io/v1alpha1
metadata:
  name: c1
spec:
  self: true
EOF
```

After a minute, check that virtual nodes named `admiralty-cluster-c1` and `admiralty-cluster-c2` have been created in cluster1:

```bash
kubectl --context "$CLUSTER1" get node -l virtual-kubelet.io/provider=admiralty
```

### Multi-Cluster Deployment in Source Cluster

Multicluster-scheduler's pod admission controller operates in namespaces labeled with `multicluster-scheduler=enabled`. In cluster1, label the `default` namespace (any other namespace could be used as well, but you would have to change the example accordingly, of course):

```bash
kubectl --context "$CLUSTER1" label namespace default multicluster-scheduler=enabled
```

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

<details>
  <summary>ℹ If you do not have direct access to the target cluster, ...</summary>
  
  ask for help from someone who does. While launching and executing the pods requires only access to the source cluster (i.e., cluster1), the above check requires access to both the source and target clusters.

</details>

### Advanced Scheduling

Multicluster-scheduler supports standard Kubernetes scheduling constraints including node selectors, affinities, etc. It will ensure that delegate pods target clusters that have nodes that match those constraints. For example, your nodes may be labeled with `failure-domain.beta.kubernetes.io/region` or `topology.kubernetes.io/region` (among other [common labels](https://kubernetes.io/docs/reference/kubernetes-api/labels-annotations-taints/)).

```sh
kubectl --context "$CLUSTER1" get nodes --show-labels
kubectl --context "$CLUSTER2" get nodes --show-labels
```

<details>
  <summary>ℹ If your test setup doesn't have region labels, ...</summary>
  
  you can add some:

  ```sh
  kubectl --context "$CLUSTER1" label nodes -l virtual-kubelet.io/provider!=admiralty topology.kubernetes.io/region=us
  kubectl --context "$CLUSTER2" label nodes -l virtual-kubelet.io/provider!=admiralty topology.kubernetes.io/region=eu
  ```

</details>

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

## Community

Need help to install/use multicluster-scheduler or integrate it with your stack? Found a bug? Or perhaps you'd like to request or even contribute a feature. Please [file an issue](https://github.com/admiraltyio/multicluster-scheduler/issues/new/choose) or talk to us on [Admiralty's community chat](https://mattermost.admiralty.io).
