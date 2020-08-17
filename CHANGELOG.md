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

## v0.10.0

### New Features

- The Source CRD and controller make it easy to create service accounts and role bindings for source clusters in a target cluster. (PR #48)
- The Target CRD and controller allow defining targets of a source cluster at runtime, rather than as Helm release values. (PR #49)

### Bugfixes

- Fix name collisions and length issues (PR #56)
- Fix cross-cluster references when parent-child names differ and parent name ins longer than 63 characters, including proxy-delegate pods (PR #57)
- Fix source cluster role references (PR #58)

See further changes already listed for release candidates below.

## v0.10.0-rc.1

This release fixes cluster summary RBAC for namespaced targets. (ClusterSummary is a new CRD introduced in v0.10.0-rc.0.)

## v0.10.0-rc.0

This release fixes a couple of bugs, one [with GKE route-based clusters](https://github.com/admiraltyio/multicluster-scheduler/issues/44) (vs.VPC-native), the other [with DNS horizontal autoscaling](https://github.com/admiraltyio/multicluster-scheduler/issues/43). As a side benefit, virtual nodes capacities and allocatable resources aren't dummy high values anymore, but the sum of the corresponding values over the nodes of the target clusters that they represent. We slipped in a small UX change: when you run `kubectl get nodes`, the role column will now say "cluster" for virtual nodes, rather than "agent", to help understand concepts. Last but not least, we're upgrading internally from Kubernetes 1.17 to 1.18.

## v0.9.3

### Bugfixes

- Fix [#38](https://github.com/admiraltyio/multicluster-scheduler/issues/38). Cross-cluster garbage collection finalizers were added to all config maps and secrets, although only those that are copied across clusters actually need them. Finalizers are removed by the controller manager when config maps and secrets are terminating, so the bug wasn't major, but it did introduce unnecessary risk, because, if the controller manager went down, config maps and secrets couldn't be deleted. It could also conflict with third-party controllers of those config maps and secrets. The fix only applies finalizers to config maps and secrets that are referrred to by multi-cluster pods, and removes extraneous finalizers (no manual cleanup needed).

## v0.9.2

### Bugfixes

- The feature introduced in v0.9.0 (config maps and secrets follow pods) wasn't compatible with namespaced targets.

## v0.9.1

### Bugfixes

- f4b1936 removed proxy pod filter on feedback controller, which crashed the controller manager if normal pods were scheduled on nodes whose names were shorter than 10 characters, and added finalizers to normal pods (manual cleanup necessary!).

## v0.9.0

### New Features

- Fix [#32](https://github.com/admiraltyio/multicluster-scheduler/issues/32). Config maps and secrets now follow pods. More specifically, if a proxy pod refers to config maps or secrets to be mounted as volumes, projected volumes, used as environment variables or, for secrets, as image pull secrets, Admiralty copies those config maps or secrets to the target cluster where the corresponding delegate pod runs.

## v0.8.2

Note: we're skipping v0.8.1 because the 0.8.1 image tag was erroneously used for a pre-release version.

### Bugfixes

- Fix [#20](https://github.com/admiraltyio/multicluster-scheduler/issues/20). Scheduling was failing altogether if the namespace didn't exist in one of the target clusters. That cluster is now simply filtered out.
- Fix [#21](https://github.com/admiraltyio/multicluster-scheduler/issues/21). The feedback controller wasn't compatible with namespaced targets. It was trying to watch remote pod chaperons at the cluster level, which wasn't allowed by remote RBAC.
- Fix [#25](https://github.com/admiraltyio/multicluster-scheduler/issues/25). The Helm chart values structure was broken, making it difficult to set resource requests/limits.
- Fix [#26](https://github.com/admiraltyio/multicluster-scheduler/issues/26). Init containers weren't stripped from their service account token volume mounts as pods were delegated.
- Fix a race condition that allowed candidate pod chaperons and their pods to be orphaned if scheduling failed. Finalizer is now added to proxy pods at admission vs. asynchronously by the feedback controller.

### Breaking Changes

- Some Helm chart values were broken (see above). As we fixed them, we reorganized all values, so some values that did work now work differently.

## v0.8.0

This release removes the central scheduler, replaced by a decentralized algorithm creating candidate pods in all targets (of which only one becomes the proxy pod's delegate). See the [proposal](proposals/decentralized.md) for details.

### New Features

- Advanced scheduling: all Kubernetes standard scheduling constraints are now respected, not just node selectors, but affinities, etc., because the candidate scheduler uses the Kubernetes [scheduling framework](https://github.com/kubernetes/enhancements/blob/master/keps/sig-scheduling/20180409-scheduling-framework.md).
- There's now one virtual node per target, making it possible to drain clusters, e.g., for blue/green cluster upgrades.
- Delegate pods are now controlled by intermediate pod chaperons. This was a technical requirement of the new scheduling algorithm, with the added benefit that if a delegate pod dies (e.g., is evicted) while its cluster is offline, a new pod will be created to replace it.

### Breaking Changes

- _Invitations_ have been removed. The user is responsible for creating service accounts for sources in target clusters. Only the `multicluster-scheduler-source` cluster role is provided, which can be bound to service accounts with cluster-scoped or namespaced role bindings.
- You can no longer enforce placement with the `multicluster.admiralty.io/clustername` annotation. Use a more idiomatic node selector instead.

### Internals

- We've started internalizing controller runtime logic (for the new feedback and pod chaperon controllers), progressively decoupling multicluster-scheduler from [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime) and [multicluster-controller](https://github.com/admiraltyio/multicluster-controller), to be more agile.

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
