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
