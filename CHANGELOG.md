# Changelog

<!--## vX.Y.Z

### New Features

- 

### Bugfixes

- 

### Breaking Changes

- 

### Internals

- 

-->

## v0.7.0

This release simplifies the design of multicluster-scheduler to enable new features: removes observations and decisions (and all the finalizers that came with them), replaces _federations_ (two-way sharing) by _invitations_ (one-way sharing, namespaced), adds support for node selectors.

### Breaking Changes

- Because the scheduler now watches and creates resources in the member clusters directly, rather than via observations and decisions, those clusters' Kubernetes APIs must be routable from the scheduler. If your clusters are private, please file an issue. We could fix that with tunnels.
- The multi-federation feature was removed and replaced by invitations.
- "Cluster namespaces" (namespaces in the scheduler's cluster that held observations and decisions) are now irrelevant.

### New Features

- Support for standard Kubernetes node selectors. If a multi-cluster pod has a node selector, multicluster-scheduler will ensure that the target cluster has nodes that match the selector.
- Invitations. Each agent specifies which clusters are allowed to create/update/delete delegate pods and services in its cluster and optionally in which namespaces. The scheduler respects invitations in its decisions, and isn't even authorized to bypass them.

### Bugfixes

- Fix [#17](https://github.com/admiraltyio/multicluster-scheduler/issues/17). The agent pod could not be restarted because it had a finalizer AND was responsible for removing its own finalizer. The agent doesn't have a finalizer anymore (because we got rid of observations).
- Fix [#16](https://github.com/admiraltyio/multicluster-scheduler/issues/16). Proxy pods were stuck in `Terminating` phase. That's because they had a non-zero graceful deletion period. On a normal node, it would be the kubelet's responsibility to delete them with a period of 0 once all containers are stopped. On a virtual-kubelet node, it is the implementation's responsibility to do that. Our solution is to mutate the graceful deletion period of proxy pods to 0 at admission, because the cross-cluster garbage collection pattern with finalizers is sufficient to ensure that they are deleted only after their delegates are deleted (and the delegates' containers are stopped).

## v0.6.0

### New Features

- Multicluster-scheduler is now a virtual-kubelet provider. Proxy pods are scheduled to a virtual node rather than actual nodes:
  - Their containers no longer need to be replaced by dummy signal traps, because they aren't run locally.
  - A proxy pod's status is simply the copy of its delegate's status. Therefore, it appears pending as long as its delegate is pending (fixes [#7](https://github.com/admiraltyio/multicluster-scheduler/issues/7)).
  - Finally, proxy pods no longer count toward the pod limit of actual nodes.
- Merge `cmd/agent` and `cmd/webhook`, and run virtual-kubelet as part of the same process. This reduces the number of Kubernetes deployments and Helm subcharts. If we need to scale them independently in the future, we can easily split them again.

### Bugfixes

- Add missing post-delete Helm hook for scheduler resources, to delete finalizers on pod and service observations and decisions.

### Internals

- Upgrade controller-runtime to v0.4.0, because v0.1.12 wasn't compatible with virtual-kubelet (their dependencies weren't). As a result, **[cert-manager](https://cert-manager.io/) is now a pre-requisite**, because certificate generation for webhooks has been removed as of controller-runtime v0.2.0.

## v0.5.0

### New Features

- Sensible defaults to make configuration easier:
  - `agent.remotes[0].secretName=remote` in Helm chart
  - clusterNamespace defaults to cluster name in Helm chart and `pkg/config`

### Bugfixes

- Fix [#13](https://github.com/admiraltyio/multicluster-scheduler/issues/13): Post-delete hook job ensures finalizers used for cross-cluster garbage collection are removed when multicluster-scheduler is uninstalled. Previously, those finalizers had to be removed by the user.

### Internals

- Align chart (and subcharts) versioning with main project for simplicity, because they are released together.

### Documentation

- [Helm chart documentation](charts/multicluster-scheduler/README.md)

## v0.4.0

### New Features

- Helm charts (v3) for an easier and more flexible installation
- Multi-federation that works!
- Better RBAC with cluster namespaces: as an option, you can setup multicluster-scheduler so that each member cluster has a dedicated namespace in the scheduler cluster for observations and decisions. This makes it possible for partially trusted clusters to participate in the same federation (they can send pods to one another, via the scheduler, but they cannot observe one another).
- More observations (to support non-basic schedulers, including [Admiralty's advanced scheduler]())

### Bugfixes

- Don't spam the log with update errors like "the object has been modified; please apply your changes to the latest version and try again". The controller would back off and retry, so the log was confusing. We now just ignore the error and let the cache enqueue a reconcile request when it receives the latest version (no back-off, but for that, the controller must watch the updated resource).

### Internals

- Use `gc` pattern from [multicluster-controller](https://github.com/admiraltyio/multicluster-controller) for cross-cluster/cross-namespace garbage collection with finalizers for `send`, `receive`, and `bind` controllers. (As a result, the local `EnqueueRequestForMulticlusterController` handler in `receive` was deleted. It now exists in multicluster-controller.)
- Split `bind` controller from `schedule` controller, to more easily plug in custom schedulers.
- `send` controller uses `unstructured` to support more observations.
- Switch to Go modules
- Faster end-to-end tests with [KIND (Kubernetes in Docker)](https://kind.sigs.k8s.io/) rather than GKE.
- Stop using skaffold (build images once in `build.sh`) and kustomize (because we now use Helm).
- Split `delegatestate` controller (in scheduler manager) from `feedback` controller (in agent manager) to make cluster namespace feature possible (where cluster1 cannot see observations from cluster2).
- The source cluster name of an observation is either its namespace (in cluster namespace mode) or the cross-cluster GC label `multicluster.admiralty.io/parent-clusterName`. The target cluster name of a decision is either its namespace (in cluster namespace mode) or the `multicluster.admiralty.io/clustername` annotation (added by `bind` and `globalsvc`). `status.liveState.metadata.ClusterName` is not longer used, except when it's backfilled in memory at the interface with the basic scheduler, which still uses the field internally.
