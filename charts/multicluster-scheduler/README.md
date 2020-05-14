# Multicluster-Scheduler Helm Chart

Multicluster-Scheduler is a system of Kubernetes controllers that intelligently schedules workloads across clusters. For general information about the project, see the [main README](../../README.md).

> Note: This chart was built for Helm v3.

> Note: multicluster-scheduler doesn't support clusters without RBAC. There's no `rbac.create` parameter.

## Prerequisites

- Kubernetes v1.17 or 1.18 (unless you build from source on a fork k8s.io/kubernetes, cf. [#19](https://github.com/admiraltyio/multicluster-scheduler/issues/19))
- [cert-manager](https://cert-manager.io/docs/installation/kubernetes/) v0.11+ in each member cluster

## Installation

Add the Admiralty Helm chart repository to your Helm client:

```sh
helm repo add admiralty https://charts.admiralty.io
helm repo update
```

Create a file named `values.yaml` containing something like:

```yaml
clusterName: CLUSTER1
targetSelf: true # e.g., for cloud bursting
targets:
  - name: CLUSTER2
    namespaced: true
      # if CLUSTER2 only allows CLUSTER1 in a specific namespace
      # (defined in the mounted kubeconfig secret)
  # ... Add more targets.
  # (You can also do that later and `helm upgrade`.)

# ... Configure the deployments with custom values:
# image overrides, node selector, resource requests and limits,
# security context, affinities and tolerations,
# cf. Parameters below.
```

> Note: The cluster names need NOT match context names and/or any other cluster nomenclature you may use. They are contained within the multicluster-scheduler [domain](https://en.wikipedia.org/wiki/Domain-driven_design).

Create a namespace for multicluster-scheduler and install the chart in it, using the values file above:

```sh
kubectl create namespace admiralty
helm install multicluster-scheduler admiralty/multicluster-scheduler \
  --version 0.8.0 \
  -n admiralty \
  -f values-scheduler.yaml
```

Repeat for other clusters.

You now need to connect your clusters. See [Bootstrapping](#bootstrapping).

## Bootstrapping

See [getting started guide](../../README.md#getting-started) for now.

> Note: If the clusters are controlled by different entities, you can save the result of `kubemcsa export` and send it securely to the other cluster administrator, for them to run `kubectl apply`.

## Post-Delete Hook

Multicluster-scheduler uses [finalizers](https://kubernetes.io/docs/tasks/access-kubernetes-api/custom-resources/custom-resource-definitions/#finalizers) for [cross-cluster garbage collection](https://twitter.com/adrienjt/status/1199467878015066112). In particular, it adds finalizers to proxy pods and global services. The finalizers block the deletion of those objects until multicluster-scheduler's scheduler deletes their delegates in other clusters. If the scheduler stopped running and those finalizers weren't removed, object deletions would be blocked indefinitely. Therefore, when multicluster-scheduler is uninstalled, a Kubernetes job will run as a [Helm post-delete hook](https://helm.sh/docs/topics/charts_hooks/) to clean up the finalizers.

## Advanced Use Cases

### Multicluster-Service-Account

If you've deployed [multicluster-service-account](https://github.com/admiraltyio/multicluster-service-account) in your clusters and bootstrapped them to import service accounts from one another, you don't need to bootstrap multicluster-scheduler imperatively (with `kubemcsa export`, see above). Instead, you can declare a service account import for each target, and refer to it in the values file.

```yaml
# values.yaml
# ...
targets:
  - serviceAccountImportName: CLUSTER2
# ...
```

```yaml
apiVersion: multicluster.admiralty.io/v1alpha1
kind: ServiceAccountImport
metadata:
  name: CLUSTER2
  namespace: admiralty
spec:
  clusterName: CLUSTER2 # name according to multicluster-service-account
  namespace: admiralty
  name: CLUSTER1
```

Don't forget to label multicluster-scheduler's namespace (e.g., "admiralty") with `multicluster-service-account=enabled` for the service account imports to be automounted.

## Parameters

| Key | Type | Default | Comment |
| --- | --- | --- | --- |
| nameOverride | string | `""` | Override chart name in object names and labels |
| fullnameOverride | string | `""` | Override chart and release names in object names |
| clusterName | string | `""` | required |
| targetSelf | bool | `false` |  |
| targets | array | `[]` |  |
| targets[].name | string | `""` | required |
| targets[].secretName | string | `""` | either secretName or serviceAccountImportName must be set |
| targets[].serviceAccountImportName | string | `""` | either secretName or serviceAccountImportName must be set, cf. [Multicluster-Service-Account](#multicluster-service-account) |
| targets[].key | string | `"config"` | if using a custom kubeconfig secret, override the secret key |
| targets[].context | string | `""` | if using a custom kubeconfig secret, with multiple contexts, override the kubeconfig's current context |
| imagePullSecretName | string | `""` |  |
| agent.image.repository | string | `"quay.io/admiralty/multicluster-scheduler-agent"` |  |
| agent.controllerManager.image.tag | string | `"0.8.0"` |  |
| agent.controllerManager.image.pullPolicy | string | `"IfNotPresent"` |  |
| agent.controllerManager.resources | object | `{}` |  |
| agent.scheduler.image.repository | string | `"quay.io/admiralty/multicluster-scheduler-scheduler"` |  |
| agent.scheduler.image.tag | string | `"0.8.0"` |  |
| agent.scheduler.image.pullPolicy | string | `"IfNotPresent"` |  |
| agent.scheduler.resources | object | `{}` |  |
| agent.nodeSelector | object | `{}` |  |
| agent.securityContext | object | `{}` |  |
| agent.affinity | object | `{}` |  |
| agent.tolerations | array | `[]` |  |
| postDeleteJob.image.repository | string | `"quay.io/admiralty/multicluster-scheduler-remove-finalizers"` |  |
| postDeleteJob.image.tag | string | `"0.8.0"` |  |
| postDeleteJob.image.pullPolicy | string | `"IfNotPresent"` |  |
| postDeleteJob.resources | object | `{}` |  |
| postDeleteJob.nodeSelector | object | `{}` |  |
| postDeleteJob.securityContext | object | `{}` |  |
| postDeleteJob.affinity | object | `{}` |  |
| postDeleteJob.tolerations | array | `[]` |  |
