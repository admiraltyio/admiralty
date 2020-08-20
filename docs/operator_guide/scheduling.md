# Scheduling

If your cluster needs to send pods to another cluster, you should create a [Target or ClusterTarget](#targets-and-cluster-targets) object.

If your cluster needs to receive pods from another cluster, you should create a [Source or ClusterSource](#sources-and-cluster-sources) object.

## Targets and Cluster Targets

ClusterTargets and Targets are Kubernetes custom resources installed with the Admiralty operator:

- A cluster-scoped ClusterTarget references a kubeconfig secret (name and namespace; can be in any namespace, though it makes sense to store these in the Admiralty operator's installation namespace, e.g., `admiralty`). The Admiralty operator will use the kubeconfig to interact with resources in the target cluster at the cluster scope, so it must be authorized to do so there (i.e., the target cluster must have a ClusterSource authorizing the kubeconfig's identity, see below).

- A namespaced Target references a kubeconfig secret in the same namespace. The Admiralty operator will use the kubeconfig to interact with resources in the target cluster in that namespace (except to view the cluster-scoped ClusterSummary, as explained below), so it must be authorized to do so there (i.e., the target cluster must have a Source authorizing the kubeconfig's identity in that namespace).

Depending on your use case, you may define a mix of ClusterTargets and Targets.

```yaml
apiVersion: multicluster.admiralty.io/v1alpha1
kind: ClusterTarget
metadata:
  name: target-cluster
spec:
  kubeconfigSecret:
    namespace: namespace-a
    name: target-cluster
---
apiVersion: multicluster.admiralty.io/v1alpha1
kind: Target
metadata:
  name: target-cluster
  namespace: namespace-a
spec:
  kubeconfigSecret:
    name: target-cluster
```

ClusterTargets and Targets can also target their local cluster, e.g., for cloud bursting. In that case, they don't reference a kubeconfig secret, but specify `spec.self=true` instead.

```yaml
apiVersion: multicluster.admiralty.io/v1alpha1
kind: ClusterTarget
metadata:
  name: this-cluster
spec:
  self: true
---
apiVersion: multicluster.admiralty.io/v1alpha1
kind: Target
metadata:
  name: this-cluster
  namespace: namespace-a
spec:
  self: true
```

## Sources and Cluster Sources

ClusterSources and Sources are custom resources installed with Admiralty:

- A cluster-scoped ClusterSource cluster-binds the source cluster role to a user or service account (in any namespace).

- A namespaced Source namespace-binds the source cluster role to a user, or a service account in the same namespace.

In either case, the cluster summary viewer cluster role is cluster-bound to that user or service account, because clustersummaries is a cluster-scoped resource. Though have no fear: all it allows is get/list/watch ClusterSummaries, which are cluster singletons that only contain the sum of the capacities and allocatable resources of their respective clusters' nodes.

If the referred service account doesn't exist, it is created.

We do not recommend storing Kubernetes service account tokens (valid forever) outside of their own clusters, which is why we came up with Identities and TrustedIdentityProviders for [cross-cluster authentication](authentication.md). From the perspective of the Kubernetes API server, Identities are Kubernetes users.

Following the example in that section, to receive pods from a cluster at the cluster level, you would create this ClusterSource:

```yaml
apiVersion: multicluster.admiralty.io/v1alpha1
kind: ClusterSource
metadata:
  name: source-cluster
spec:
  userName: spiffe://source-cluster/ns/namespace-a/id/default
```

To receive pods in the `namespace-a` namespace only, you would create this Source:

```yaml
apiVersion: multicluster.admiralty.io/v1alpha1
kind: Source
metadata:
  name: source-cluster
  namespace: namespace-a
spec:
  userName: spiffe://source-cluster/ns/namespace-a/id/default
```
