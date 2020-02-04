# Multicluster-Scheduler Helm Chart

Multicluster-Scheduler is a system of Kubernetes controllers that intelligently schedules workloads across clusters. For general information about the project, see the [main README](../../README.md).

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
  --wait \
  jetstack/cert-manager
```

Then:

```sh
helm repo add admiralty https://charts.admiralty.io
helm repo update

# in member clusters
kubectl create namespace admiralty
helm install multicluster-scheduler-member admiralty/multicluster-scheduler \
  --namespace admiralty \
  --version 0.7.0 \
  --set agent.enabled=true \
  --set agent.invitations[0].clusterName=c1 \
  --set agent.invitations[0].clusterName=c2 # etc.
kubemcsa export multicluster-scheduler-member-agent-for-scheduler \
  -n admiralty --as c1 > c1.yaml
# repeat for c2, etc.

# in the scheduler's cluster (which could also be a member)
kubectl create namespace admiralty # if not already created
helm install multicluster-scheduler admiralty/multicluster-scheduler \
  --namespace admiralty \
  --version 0.7.0 \
  --set scheduler.enabled=true \
  --set scheduler.clusters[0].name=c1 \
  --set scheduler.clusters[1].name=c2 # etc.

kubectl apply -n admiralty -f c1.yaml
kubectl apply -n admiralty -f c2.yaml
# etc.
```

## Subcharts

This chart is composed of two subcharts: `scheduler` is installed in the scheduler's cluster; `agent` is installed in each member cluster. The scheduler's cluster can be a member cluster too, in which case you'll install both subcharts in it.

The `scheduler` and `agent` subcharts each install a deployment and its associated config map, service account, and [RBAC](https://kubernetes.io/docs/reference/access-authn-authz/rbac/) resources. Both are namespaced. The `agent` subchart also installs admission-webhook-related resources: service, configuration, cert-manager self-signed issuer and certificate.

> Note: multicluster-scheduler doesn't support clusters without RBAC. There's no `rbac.create` parameter.

## Installation

Add the Admiralty Helm chart repository to your Helm client:

```sh
helm repo add admiralty https://charts.admiralty.io
helm repo update
```

All subcharts are disabled by default, so you CANNOT just `helm install`. That wouldn't install anything. Instead, you need to configure the chart a certain way for the scheduler's cluster, and a different way for each member cluster.

### Scheduler's Cluster

Create a file named `values-scheduler.yaml` containing something like:

```yaml
scheduler:
  enabled: true # enables the scheduler subchart

  clusters:
    - name: SCHEDULER_CLUSTER_NAME # if the scheduler's cluster is also a member cluster
    - name: MEMBER_1_CLUSTER_NAME
    - name: MEMBER_2_CLUSTER_NAME
    # ... Add more member clusters. (You can also do that later and `helm upgrade`.)

  # ... Configure the scheduler deployment with custom values
  # (image override, node selector, resource requests and limits,
  # security context, affinities and tolerations).

# Include the following if the scheduler's cluster is also a member cluster.
# Note: Alternatively, you could install two releases in that cluster (with different release names!),
# one with the scheduler and clusters subcharts, the other with the agent subchart.

agent:
  enabled: true

  invitations:
    - clusterName: MEMBER_1_CLUSTER_NAME
      namespaces:
      # in this agent's cluster, MEMBER_1_CLUSTER_NAME will only be allowed 
      # to create delegate pods and services in these namespace
        - SHARED_NAMESPACE
    - clusterName: MEMBER_2_CLUSTER_NAME
      # if no namespace if specified, invitation is valid for ALL namespaces

  # ... Configure the agent deployment with custom values.
```

> Note: The cluster names need NOT match context names and/or any other cluster nomenclature you may use. They are contained within the multicluster-scheduler [domain](https://en.wikipedia.org/wiki/Domain-driven_design).

With your kubeconfig and context pointing at the scheduler's cluster (with the usual `KUBECONFIG` environment variable, `--kubeconfig` and/or `--context` option), create a namespace for multicluster-scheduler and install the chart in it, using the values file above:

```sh
kubectl create namespace admiralty
helm install multicluster-scheduler admiralty/multicluster-scheduler \
  --version 0.7.0 \
  -f values-scheduler.yaml -n admiralty
```

### Member Clusters

Create another file named `values-member-1.yaml` containing something like:

```yaml
agent:
  enabled: true

  invitations:
    - clusterName: MEMBER_1_CLUSTER_NAME
      namespaces:
      # in this agent's cluster, MEMBER_1_CLUSTER_NAME will only be allowed 
      # to create delegate pods and services in these namespace
        - SHARED_NAMESPACE
    - clusterName: MEMBER_2_CLUSTER_NAME
      # if no namespace if specified, ALL namespaces are allowed

  # ... Configure the agent deployment with custom values.
```

With your kubeconfig and context pointing at that member cluster, create a namespace for multicluster-scheduler and install the chart in it, using the values file above:

```sh
kubectl create namespace admiralty
helm install multicluster-scheduler admiralty/multicluster-scheduler \
  --version 0.7.0 \
  -f values-member-1.yaml -n admiralty
```

Repeat for other member clusters.

You now need to connect your clusters. See [Bootstrapping](#bootstrapping).

## Bootstrapping

The `agent` subchart created a service account named `multicluster-scheduler-agent-for-scheduler` (or `HELM_RELEASE_NAME-agent-for-scheduler`) in each member cluster, to be used remotely by the scheduler. To use a service account remotely, you generally need to know its namespace, a routable address (domain or IP) to call its cluster's Kubernetes API, a valid CA certificate for that address, and its token.

By default, multicluster-scheduler expects that information to be formatted as a standard [kubeconfig](https://kubernetes.io/docs/concepts/configuration/organize-cluster-access-kubeconfig/) file, saved in the `config` data field of a secret named after the corresponding member cluster, in the scheduler's namespace.

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

Then, for each member cluster (including the scheduler's cluster if it is a member), with your kubeconfig and context pointing at that cluster, run `kubemcsa export` to generate a secret template:

```sh
./kubemcsa export multicluster-scheduler-agent-for-scheduler -n admiralty --as MEMBER_CLUSTER_NAME > MEMBER_CLUSTER_NAME.yaml
```

Finally, with your kubeconfig and context pointing at the scheduler's cluster, for each generated secret template:

```sh
kubectl apply -n admiralty -f MEMBER_CLUSTER_NAME.yaml
```

> Note: If the scheduler's cluster and member clusters are controlled by different entities, you can save the result of `kubemcsa export`, send it securely to the scheduler's cluster administrator, for them to run `kubectl apply`.

## Adding Member Clusters

Install and bootstrap the new member clusters as you did for the original ones. Then, edit the scheduler's values file, adding items to `scheduler.clusters`:

```yaml
scheduler:
  # ...
  clusters:
  # ...
  - name: NEW_MEMBER_CLUSTER_NAME
  # ...

# ...
```

With your kubeconfig and context pointing at the scheduler's cluster, run `helm upgrade`, using the edited values file:

```sh
helm upgrade multicluster-scheduler admiralty/multicluster-scheduler \
  --version 0.7.0 \
  -f values-scheduler.yaml -n admiralty
```

> Note: You can also create a partial values file containing `scheduler.clusters` only, and run `helm upgrade` with the `--reuse-values` option.

Also make sure the new cluster is invited by other clusters (edit their values files and upgrade their Helm releases).

## Post-Delete Hook

Multicluster-scheduler uses [finalizers](https://kubernetes.io/docs/tasks/access-kubernetes-api/custom-resources/custom-resource-definitions/#finalizers) for [cross-cluster garbage collection](https://twitter.com/adrienjt/status/1199467878015066112). In particular, it adds finalizers to proxy pods and global services. The finalizers block the deletion of those objects until multicluster-scheduler's scheduler deletes their delegates in other clusters. If the scheduler stopped running and those finalizers weren't removed, object deletions would be blocked indefinitely. Therefore, when multicluster-scheduler is uninstalled, a Kubernetes job will run as a [Helm post-delete hook](https://helm.sh/docs/topics/charts_hooks/) to clean up the finalizers.

## Advanced Use Cases

### Multicluster-Service-Account

If you've deployed [multicluster-service-account](https://github.com/admiraltyio/multicluster-service-account) in the member clusters and bootstrapped them to import service accounts from the scheduler's cluster, you don't need to bootstrap multicluster-scheduler imperatively (with `kubemcsa export`, see above). Instead, you can declare a service account import for each member cluster in the scheduler's cluster, and refer to it in the scheduler's values file.

```yaml
# values-scheduler.yaml
scheduler:
  # ...
  clusters:
  - serviceAccountImportName: MEMBER_CLUSTER_NAME
```

```yaml
# standard mode
apiVersion: multicluster.admiralty.io/v1alpha1
kind: ServiceAccountImport
metadata:
  name: MEMBER_CLUSTER_NAME
  namespace: admiralty
spec:
  clusterName: MEMBER_CLUSTER_NAME # according to multicluster-service-account
  namespace: admiralty
  name: multicluster-scheduler-agent-for-scheduler
```

Don't forget to label the scheduler's namespace (e.g., "admiralty") with `multicluster-service-account=enabled` for the service account imports to be automounted.

## Parameters

| Key | Type | Default | Comment |
| --- | --- | --- | --- |
| global.nameOverride | string | `""` | Override chart name in object names and labels |
| global.fullnameOverride | string | `""` | Override chart and release names in object names |
| global.postDeleteJob.image.repository | string | `"quay.io/admiralty/multicluster-scheduler-remove-finalizers"` |  |
| global.postDeleteJob.image.tag | string | `"0.7.0"` |  |
| global.postDeleteJob.image.pullPolicy | string | `"IfNotPresent"` |  |
| global.postDeleteJob.image.pullSecretName | string | `""` |  |
| global.postDeleteJob.nodeSelector | object | `{}` |  |
| global.postDeleteJob.resources | object | `{}` |  |
| global.postDeleteJob.securityContext | object | `{}` |  |
| global.postDeleteJob.affinity | object | `{}` |  |
| global.postDeleteJob.tolerations | array | `[]` |  |
| agent.enabled | boolean | `false` |  |
| agent.invitations[].clusterName | string | `""` | required |
| agent.invitations[].namespaces | []string | `[]` | if empty, invitation is valid for ALL namespaces |
| agent.image.repository | string | `"quay.io/admiralty/multicluster-scheduler-agent"` |  |
| agent.image.tag | string | `"0.7.0"` |  |
| agent.image.pullPolicy | string | `"IfNotPresent"` |  |
| agent.image.pullSecretName | string | `""` |  |
| agent.nodeSelector | object | `{}` |  |
| agent.resources | object | `{}` |  |
| agent.securityContext | object | `{}` |  |
| agent.affinity | object | `{}` |  |
| agent.tolerations | array | `[]` |  |
| scheduler.enabled | boolean | `false` |  |
| scheduler.clusters | array | `[]` |  |
| scheduler.clusters[].name | string | `""` | required |
| scheduler.clusters[].secretName | string | `""` | either secretName or serviceAccountImportName must be set |
| scheduler.clusters[].serviceAccountImportName | string | `""` | either secretName or serviceAccountImportName must be set, cf. [Multicluster-Service-Account](#multicluster-service-account) |
| scheduler.clusters[].key | string | `"config"` | if using a custom kubeconfig secret, override the secret key |
| scheduler.clusters[].context | string | `""` | if using a custom kubeconfig secret, with multiple contexts, override the kubeconfig's current context |
| scheduler.image.repository | string | `"quay.io/admiralty/multicluster-scheduler-basic"` |  |
| scheduler.image.tag | string | `"0.7.0"` |  |
| scheduler.image.pullPolicy | string | `"IfNotPresent"` |  |
| scheduler.image.pullSecretName | string | `""` |  |
| scheduler.nodeSelector | object | `{}` |  |
| scheduler.resources | object | `{}` |  |
| scheduler.securityContext | object | `{}` |  |
| scheduler.affinity | object | `{}` |  |
| scheduler.tolerations | array | `[]` |  |
