# Multicluster-Scheduler Helm Chart

Multicluster-Scheduler is a system of Kubernetes controllers that intelligently schedules workloads across clusters. For general information about the project (getting started guide, how it works, etc.), see the [main README](../../README.md).

> Note: This chart was built for Helm v3.

## Prerequisites

- [cert-manager](https://cert-manager.io/docs/installation/kubernetes/) v0.11+ in each member cluster

## TL;DR

If you haven't already installed cert-manager:

```sh
helm repo add jetstack https://charts.jetstack.io
helm repo update

# in member clusters
kubectl apply --validate=false -f https://raw.githubusercontent.com/jetstack/cert-manager/release-0.12/deploy/manifests/00-crds.yaml
kubectl create namespace cert-manager
helm install cert-manager \
  --namespace cert-manager \
  --version v0.12.0 \
  jetstack/cert-manager
```

This will install multicluster-scheduler in the default namespace, in [standard mode](#standard-mode-vs-cluster-namespaces):

```sh
helm repo add admiralty https://charts.admiralty.io
helm repo update

# in the scheduler's cluster
helm install multicluster-scheduler admiralty/multicluster-scheduler \
  --set global.clusters[0].name=c1 \
  --set global.clusters[1].name=c2 \
  --set scheduler.enabled=true \
  --set clusters.enabled=true
kubemcsa export c1 --as remote > c1.yaml
kubemcsa export c2 --as remote > c2.yaml

# in member clusters
helm install multicluster-scheduler-member admiralty/multicluster-scheduler \
  --set agent.enabled=true \
  --set agent.clusterName=c1 \
kubectl apply -f c1.yaml
# repeat for c2
```

## Subcharts

This chart is composed of three subcharts: `scheduler` and `clusters` are installed in the scheduler's cluster; `agent` is installed in each member cluster. The scheduler's cluster can be a member cluster too, in which case you'll install all three subcharts in it.

The `scheduler` and `agent` subcharts each install a deployment and its associated config map, service account, and [RBAC](https://kubernetes.io/docs/reference/access-authn-authz/rbac/) resources. Both are namespaced. The `agent` subchart also installs admission-webhook-related resources: service, configuration, cert-manager self-signed issuer and certificate.

The `clusters` subchart doesn't install any deployment, but service accounts and RBAC resources, for the agents in each member cluster to call the Kubernetes API of the scheduler's cluster (cf. [How It Works](../../README.md) in the main README). In [cluster-namespace mode](#standard-mode-vs-cluster-namespaces), the `clusters` subchart installs resources in multiple namespaces; in standard mode, it must be installed in the same namespace as the scheduler.

> Note: multicluster-scheduler doesn't support clusters without RBAC. There's no `rbac.create` parameter.

## Standard Mode vs. Cluster Namespaces

Before you install multicluster-scheduler, you need to think about security and decide whether

- a) you want all member clusters to send observations and receive decisions to/from the same namespace in the scheduler's cluster, typically where the `scheduler` subchart is installed, or 

- b) to/from "cluster namespaces", i.e., namespaces in the scheduler's cluster whose only purpose is to host observations and decisions for individual member clusters, allowing access control by cluster.

The former option ("standard mode") is the simplest and works well in situations where all member clusters trust each other not to spy on, nor tamper with, each other's observations and decisions.

In situations where member clusters only trust the scheduler's cluster but not each other in that regard (though, of course, they trust each other enough to schedule workloads to one another), cluster namespaces are preferred.

## Installation

Add the Admiralty Helm chart repository to your Helm client:

```sh
helm repo add admiralty https://charts.admiralty.io
helm repo update
```

All subcharts are disabled by default, so you CANNOT just `helm install`. That wouldn't install anything. Instead, you need to configure the chart a certain way for the scheduler's cluster, and a different way for each member cluster.

### Standard Mode

#### Scheduler's Cluster

Create a file named `values-scheduler.yaml` containing something like:

```yaml
global: # parameters shared by subcharts
  clusters:
    - name: SCHEDULER_CLUSTER_NAME # if the scheduler's cluster is also a member cluster
    - name: MEMBER_1_CLUSTER_NAME
    - name: MEMBER_2_CLUSTER_NAME
    # ... Add more member clusters. (You can also do that later and `helm upgrade`.)

scheduler:
  enabled: true # enables the scheduler subchart
  # ... Configure the scheduler deployment with custom values
  # (image override, node selector, resource requests and limits,
  # security context, affinities and tolerations).

clusters:
  enabled: true # enables the clusters subchart
  # The clusters subchart doesn't have any parameters of its own.

# Include the following if the scheduler's cluster is also a member cluster.
# Note: Alternatively, you could install two releases in that cluster (with different release names!),
# one with the scheduler and clusters subcharts, the other with the agent subchart.

agent:
  enabled: true
  clusterName: SCHEDULER_CLUSTER_NAME # In standard mode, clusters declare their own names.
  # ... Configure the agent deployment with custom values.
```

> Note: The cluster names need NOT match context names and/or any other cluster nomenclature you may use. They are contained within the multicluster-scheduler [domain](https://en.wikipedia.org/wiki/Domain-driven_design).

> Note: By default, a member cluster is part of the "default" federation, unless you overwrite its `memberships`. See [Multiple Federations](#multiple-federations).

With your kubeconfig and context pointing at the scheduler's cluster (with the usual `KUBECONFIG` environment variable, `--kubeconfig` and/or `--context` option), create a namespace for multicluster-scheduler and install the chart in it, using the values file above:

```sh
kubectl create namespace admiralty
helm install multicluster-scheduler admiralty/multicluster-scheduler \
  -f values-scheduler.yaml -n admiralty
```

#### Member Clusters

Create another file named `values-member-1.yaml` containing something like:

```yaml
agent:
  enabled: true
  clusterName: MEMBER_1_CLUSTER_NAME
  # ... Configure the agent deployment with custom values.
```

With your kubeconfig and context pointing at that member cluster, create a namespace for multicluster-scheduler and install the chart in it, using the values file above:

```sh
kubectl create namespace admiralty
helm install multicluster-scheduler admiralty/multicluster-scheduler \
  -f values-member-1.yaml -n admiralty
```

For other member clusters, prepare more files with different cluster names, or use Helm's `--set` option to override the `agent.clusterName` parameter, e.g.:

```sh
kubectl create namespace admiralty
helm install multicluster-scheduler admiralty/multicluster-scheduler \
  -f values-member-1.yaml -n admiralty \
  --set agent.clusterName=MEMBER_2_CLUSTER_NAME
```

You now need to connect your clusters. See [Bootstrapping](#bootstrapping).

### Cluster-Namespace Mode

#### Scheduler's Cluster

Create a file named `values-scheduler.yaml` containing something like:

```yaml
global: # parameters shared by subcharts
  useClusterNamespaces: true
  clusters:
    - name: SCHEDULER_CLUSTER_NAME # if the scheduler's cluster is also a member cluster
      clusterNamespace: SCHEDULER_CLUSTER_NAMESPACE # if different than cluster name
    - name: MEMBER_1_CLUSTER_NAME
      clusterNamespace: MEMBER_1_CLUSTER_NAMESPACE
    - name: MEMBER_2_CLUSTER_NAME
      clusterNamespace: MEMBER_2_CLUSTER_NAMESPACE
    # ... Add more member clusters. (You can also do that later and `helm upgrade`.)

scheduler:
  enabled: true # enables the scheduler subchart
  # ... Configure the scheduler deployment with custom values
  # (image override, node selector, resource requests and limits,
  # security context, affinities and tolerations).

clusters:
  enabled: true # enables the clusters subchart
  # The clusters subchart doesn't have any parameters of its own.

# Include the following if the scheduler's cluster is also a member cluster.

agent:
  enabled: true
  # ... Configure the agent deployment with custom values.
```

> Note: The cluster names need NOT match context names and/or any other cluster nomenclature you may use. They are contained within the multicluster-scheduler [domain](https://en.wikipedia.org/wiki/Domain-driven_design).

> Note: By default, a member cluster is part of the "default" federation, unless you overwrite its `memberships`. See [Multiple Federations](#multiple-federations).

With your kubeconfig and context pointing at the scheduler's cluster (with the usual `KUBECONFIG` environment variable, `--kubeconfig` and/or `--context` option), create a namespace for multicluster-scheduler, create the cluster namespaces, and install the chart, using the values file above:

```sh
kubectl create namespace admiralty

kubectl create namespace SCHEDULER_CLUSTER_NAMESPACE # if the scheduler's cluster is also a member cluster
kubectl create namespace MEMBER_1_CLUSTER_NAMESPACE
kubectl create namespace MEMBER_2_CLUSTER_NAMESPACE
# ...

helm install multicluster-scheduler admiralty/multicluster-scheduler \
  -f values-scheduler.yaml -n admiralty
```

#### Member Clusters

In cluster-namespace mode, member clusters need not know their names, so you can use the same values file for all of them. Create another file named `values-member.yaml` containing something like:

```yaml
agent:
  enabled: true
  # ... Configure the agent deployment with custom values.
```

With your kubeconfig and context pointing at any member cluster, install the chart using that values file:

```sh
kubectl create namespace admiralty
helm install multicluster-scheduler admiralty/multicluster-scheduler \
  -f values-member.yaml -n admiralty
```

You now need to connect your clusters. See [Bootstrapping](#bootstrapping).

## Bootstrapping

The `clusters` subchart created service accounts in the scheduler's cluster, to be used remotely by the agents in the member clusters. To use a service account remotely, you generally need to know its namespace, a routable address (domain or IP) to call its cluster's Kubernetes API, a valid CA certificate for that address, and its token.

By default, multicluster-scheduler expects that information to be formatted as a standard [kubeconfig](https://kubernetes.io/docs/concepts/configuration/organize-cluster-access-kubeconfig/) file, saved in the `config` data field of a secret named `remote` in the agent's namespace.

Luckily, the `kubemcsa export` command of [multicluster-service-account](https://github.com/admiraltyio/multicluster-service-account#you-might-not-need-multicluster-service-account) can prepare the secrets for us (you don't need to deploy multicluster-service-account).

> Note: If you've deployed [multicluster-service-account](https://github.com/admiraltyio/multicluster-service-account), you can use "service account imports" instead. See the [advanced use case](#advanced-use-cases).

First, install kubemcsa:

```bash
MCSA_RELEASE_URL=https://github.com/admiraltyio/multicluster-service-account/releases/download/v0.6.1
OS=linux # or darwin (i.e., OS X) or windows
ARCH=amd64 # if you're on a different platform, you must know how to build from source
curl -Lo kubemcsa "$MCSA_RELEASE_URL/kubemcsa-$OS-$ARCH"
chmod +x kubemcsa
```

Then, with your kubeconfig and context pointing at the scheduler's cluster, for each member cluster (including the scheduler's cluster if it is a member), run `kubemcsa export` to generate a secret template:

```sh
# standard mode
./kubemcsa export MEMBER_CLUSTER_NAME --as remote > MEMBER_CLUSTER_NAME.yaml

# cluster-namespace mode
./kubemcsa export remote --namespace MEMBER_CLUSTER_NAMESPACE --as remote > MEMBER_CLUSTER_NAME.yaml
```

Finally, for each member cluster, with your kubeconfig and context pointing at that cluster:

```sh
kubectl apply -f MEMBER_CLUSTER_NAME.yaml
```

> Note: If the scheduler's cluster and member clusters are controlled by different entities, you can save the result of `kubemcsa export`, send it securely to a member cluster administrator, for them to run `kubectl apply`.

## Adding Member Clusters

To add member clusters, edit the scheduler's values file, adding items to `global.clusters`:

```yaml
global:
  # ...
  clusters:
  # ...
  - name: NEW_MEMBER_CLUSTER_NAME
    clusterNamespace: NEW_MEMBER_CLUSTER_NAMESPACE # cluster-namespace mode only, if different than cluster name
  # ...

# ...
```

With your kubeconfig and context pointing at the scheduler's cluster, run `helm upgrade`, using the edited values file. If applicable, don't forget to create the corresponding cluster namespaces beforehand:

```sh
# cluster-namespace mode only
kubectl create namespace NEW_MEMBER_CLUSTER_NAMESPACE

helm upgrade multicluster-scheduler admiralty/multicluster-scheduler \
  -f values-scheduler.yaml -n admiralty
```

> Note: You can also create a partial values file containing `global.clusters` only, and run `helm upgrade` with the `--reuse-values` option.

Then, install and bootstrap the new member clusters as you did for the original ones.

## Post-Delete Hook

Multicluster-scheduler uses [finalizers](https://kubernetes.io/docs/tasks/access-kubernetes-api/custom-resources/custom-resource-definitions/#finalizers) for [cross-cluster garbage collection](https://twitter.com/adrienjt/status/1199467878015066112). In particular, it adds finalizers to nodes, pods, and other pre-existing resources that are observed for proper scheduling. The finalizers block the deletion of nodes, pods, etc. until multicluster-scheduler's agent deletes their corresponding observations in the scheduler's cluster. If the agent stopped running and those finalizers weren't removed, object deletions would be blocked indefinitely. Therefore, when multicluster-scheduler is uninstalled, a Kubernetes job will run as a [Helm post-delete hook](https://helm.sh/docs/topics/charts_hooks/) to clean up the finalizers.

## Advanced Use Cases

### Multiple Federations

Multicluster-scheduler allows for topologies where, e.g., cluster A can send workloads to cluster B and C, but clusters B and C can only send workloads to cluster A (i.e., cluster B cannot send workloads to cluster C and vice-versa). In this example, you would set up two overlapping federations, one with clusters A and B, the other with clusters A and C. Here's what the values file would look like:

```yaml
global:
  useClusterNamespaces: true # you'd probably want to use cluster namespaces in this case
  clusters:
    - name: cluster-a
      memberships:
        - federationName: f1
        - federationName: f2
    - name: cluster-b
      memberships:
        - federationName: f1
    - name: cluster-c
      memberships:
        - federationName: f2
```

### Multicluster-Service-Account

If you've deployed [multicluster-service-account](https://github.com/admiraltyio/multicluster-service-account) in the member clusters and bootstrapped them to import service accounts from the scheduler's cluster, you don't need to bootstrap multicluster-scheduler imperatively (with `kubemcsa export`, see above). Instead, you can declare a service account import in each member cluster and refer to it in the member cluster's values file.

```yaml
# values-member.yaml
agent:
  # ...
  remotes:
  - serviceAccountImportName: remote
```

```yaml
# standard mode
apiVersion: multicluster.admiralty.io/v1alpha1
kind: ServiceAccountImport
metadata:
  name: remote
  namespace: admiralty
spec:
  clusterName: SCHEDULER_CLUSTER_NAME # according to multicluster-service-account
  namespace: admiralty
  name: MEMBER_CLUSTER_NAME
```

```yaml
# cluster-namespace mode
apiVersion: multicluster.admiralty.io/v1alpha1
kind: ServiceAccountImport
metadata:
  name: remote
  namespace: admiralty
spec:
  clusterName: SCHEDULER_CLUSTER_NAME # according to multicluster-service-account
  namespace: MEMBER_CLUSTER_NAMESPACE
  name: remote
```

Don't forget to label the agent's namespace (e.g., "admiralty") with `multicluster-service-account=enabled` for the service account import to be automounted.

## Parameters

| Key | Type | Default | Comment |
| --- | --- | --- | --- |
| global.nameOverride | string | `""` | Override chart name in object names and labels |
| global.fullnameOverride | string | `""` | Override chart and release names in object names |
| global.useClusterNamespaces | boolean | `false` | cf. [Standard Mode vs. Cluster Namespaces](#standard-mode-vs-cluster-namespaces) |
| global.clusters | array | `[]` |  |
| global.clusters[].name | string | `""` | required |
| global.clusters[].clusterNamespace | string | the cluster name in cluster-namespace mode, else the release namespace |  |
| global.clusters[].memberships | array | `[{"federationName":"default"}]` | cf. [Multiple Federations](#multiple-federations) |
| global.clusters[].memberships[].federationName | string | `""` | required |
| global.postDeleteJob.image.repository | string | `"quay.io/admiralty/multicluster-scheduler-remove-finalizers"` |  |
| global.postDeleteJob.image.tag | string | `"0.6.0"` |  |
| global.postDeleteJob.image.pullPolicy | string | `"IfNotPresent"` |  |
| global.postDeleteJob.image.pullSecretName | string | `""` |  |
| global.postDeleteJob.nodeSelector | object | `{}` |  |
| global.postDeleteJob.resources | object | `{}` |  |
| global.postDeleteJob.securityContext | object | `{}` |  |
| global.postDeleteJob.affinity | object | `{}` |  |
| global.postDeleteJob.tolerations | array | `[]` |  |
| agent.enabled | boolean | `false` |  |
| agent.clusterName | string | `""` | required in standard mode, ignored in cluster-namespace mode |
| agent.remotes | array | `[{"secretName":"remote"}]` | agents can technically connect to multiple remote schedulers (not documented yet) |
| agent.remotes[].secretName | string | `""` | either secretName or serviceAccountImportName must be set |
| agent.remotes[].serviceAccountImportName | string | `""` | either secretName or serviceAccountImportName must be set, cf. [Multicluster-Service-Account](#multicluster-service-account) |
| agent.remotes[].key | string | `"config"` | if using a custom kubeconfig secret, override the secret key |
| agent.remotes[].context | string | `""` | if using a custom kubeconfig secret, with multiple contexts, override the kubeconfig's current context |
| agent.remotes[].clusterName | string | `""` | override agent.clusterName for this remote |
| agent.image.repository | string | `"quay.io/admiralty/multicluster-scheduler-agent"` |  |
| agent.image.tag | string | `"0.6.0"` |  |
| agent.image.pullPolicy | string | `"IfNotPresent"` |  |
| agent.image.pullSecretName | string | `""` |  |
| agent.nodeSelector | object | `{}` |  |
| agent.resources | object | `{}` |  |
| agent.securityContext | object | `{}` |  |
| agent.affinity | object | `{}` |  |
| agent.tolerations | array | `[]` |  |
| clusters.enabled | boolean | `false` |  |
| scheduler.enabled | boolean | `false` |  |
| scheduler.image.repository | string | `"quay.io/admiralty/multicluster-scheduler-basic"` |  |
| scheduler.image.tag | string | `"0.6.0"` |  |
| scheduler.image.pullPolicy | string | `"IfNotPresent"` |  |
| scheduler.image.pullSecretName | string | `""` |  |
| scheduler.nodeSelector | object | `{}` |  |
| scheduler.resources | object | `{}` |  |
| scheduler.securityContext | object | `{}` |  |
| scheduler.affinity | object | `{}` |  |
| scheduler.tolerations | array | `[]` |  |
