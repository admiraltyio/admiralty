# Fully Decentralized Multi-Cluster Scheduler

## Summary

This document describes how to fully decentralize multicluster-scheduler. The central scheduler component would no longer be. Agents would take over cross-cluster control loops, while minimizing cross-cluster privileges.

## Motivation

Currently, multicluster-scheduler is composed of two components: the scheduler and the agent. The agent is deployed in each cluster and is responsible for in-cluster control loops (virtual node, service reroute). The scheduler is responsible for cross-cluster control loops between member clusters (cross-cluster pod scheduling/binding, global services) and can be deployed anywhere. Multicluster-scheduler is therefore a partially centralized system in the control plane, while decentralized in the data plane (any cluster can create source pods/services, the scheduler doesn't persist any state).

Centralization, even partial, raises several concerns:

1. The scheduler is a single point of failure. When the scheduler goes down, cross-cluster control loops are halted across all clusters. Target pods and services that are already created keep living, but can't be updated/deleted remotely, new ones can't be created, and the status of source objects isn't updated. If this was the only concern, one way to mitigate it would be to run multiple, distributed schedulers and elect a leader.

2. The scheduler watches all nodes and pods in target clusters (note: a cluster can be both a source and target), which makes it possible to reuse single-cluster scheduling algorithms.

    a) However, that also means multicluster-scheduler cannot be used as a way to escape single-cluster scheduler scale limits (e.g., numbers of global nodes and pods).

    b) Furthermore, the generated traffic across clusters might be too costly or take up too much bandwidth in some use cases [call for use cases].

3. The scheduler is given extended privileges, especially in target clusters. In source clusters, it watches source pods and services. In target clusters, it watches resources including nodes and pods in all namespaces, to inform scheduling decisions, and creates/updates/deletes target pods and services on behalf of source clusters, using impersonation for the latter.

    The scheduler must be trusted by all clusters. Target clusters can choose which source clusters to invite (i.e., to allow to CRUD pods and services) in which namespaces, but they have to trust the scheduler to respect those invitations (i.e., to impersonate as expected). This is an issue for semi-trusted organizations, e.g., universities, wanting to share compute resources: who runs the **a) almighty, b) all-knowing** scheduler?

Technically, multiple schedulers can oversee overlapping cluster groups, as long as they don't compete for the same source pods, a condition that can be guaranteed by carefully designing invitations. By extension, there can be one scheduler per cluster, given invitations only meant for itself by some or all of the other clusters. This setup is equivalent to distributing the central scheduler's responsibilities to the agents. There's no more almighty organization. However, now all organizations are all-knowing (they watch resources including nodes and pods in all namespaces in the clusters that invited them, to inform scheduling decisions). Moreover, if the invitation graph is well connected, overall traffic volume is multiplied by up to the number of clusters.

**Figure 1.** Mitigation of concerns by simply distributing the central scheduler's responsibilities to the agents. More changes are necessary.

| Concern | Mitigated :) or Worsened :( |
| --- | --- |
| 1 | :) |
| 2a | - |
| 2b | :( |
| 3a | :) |
| 3b | :( |

If we want to prevent source clusters from watching nodes and pods in all namespaces in target clusters (to mitigate 2b and 3b), we need to change how cross-cluster scheduling works. Target clusters should only surface aggregated, hence less sensitive and less verbose information for source clusters to inform their scheduling decisions. This document describes two ways to achieve that, one that respects all single-cluster scheduling constraints but doesn't mitigate 2a (proposal), one that mitigates 2a but doesn't respect all single-cluster scheduling constraints (alternative).

### Goals

- No single point of failure.
- No all-knowing, almighty intermediary.
- Minimize cross-cluster privileges.
- Simplify configuration (one component vs. two components).
- Respect all single-cluster scheduling constraints.

### Non-Goals?

- Minimize cross-cluster traffic volume. [could be a byproduct]
- Mitigate single-cluster scheduler scale limits.

## Proposal

When a source pod is created, **candidate** target pods are created in all target clusters. They are scheduled by a custom scheduler that is mostly identical to the standard Kubernetes scheduler, but includes [Scheduling Framework](https://kubernetes.io/docs/concepts/configuration/scheduling-framework/) [plugins](https://github.com/kubernetes/enhancements/blob/master/keps/sig-scheduling/20180409-scheduling-framework.md) to eventually only bind one of the candidates.

A _reserve_ plugin annotates candidate target pods to notify their source agents that they are ready to be bound.

When the first candidate target pod for a source pod is ready to be bound, the source agent annotates it to notify the target scheduler that it is allowed to be bound. The source agent "reserves" the candidate in local memory, so as not to allow following candidates for the same source pod.

A _permit_ plugin approves bindings if candidate target pods are annotated to be approved, and delays them otherwise.

A _post-bind_ plugin annotates candidate target pods to notify their source agents that they are bound. (Note: the source agents could also see that from pod statuses.)

When a candidate target pod is bound, the source agent deletes other candidates for the same source pod, whether they are waiting to be bound or not yet (if ever).

An _un-reserve_ plugin annotates candidate target pods to notify their source agents that their bindings failed.

When a candidate target pod is un-reserved, the source agent "un-reserves" the candidate in local memory, so another candidate for the same source pod can be allowed instead.

### Optional: Cluster Scoring

The process described above optimizes for scheduling speed: the first cluster to announce that it can schedule a pod binds it. Instead, one may want to score clusters, for example, to prioritize on-prem resources over elastic cloud resources.

To do that, the source agent could wait for more than one candidate to be ready to be bound and compare their scores (notified with a _normalize-score_ plugin), then allow the highest. It is unclear how long the source agent should wait: until a minimum number of candidates are ready (some candidates may never be ready if they are unschedulable), or for a certain time (how long?), whichever comes first?

Note that, technically, there's no guarantee that the scores provided by the target clusters are comparable if they are controlled by separate organizations and implement custom schedulers (see below).

### Compatibility with Custom Schedulers

Currently, multicluster-scheduler is compatible with custom schedulers, because it only selects a cluster, then lets any single-cluster scheduler select a node.

This proposal requires to ship a custom scheduler with each agent. The custom scheduler is a wrapper around the Kubernetes standard scheduler, compiled with it, but built [out of tree](https://github.com/kubernetes/enhancements/blob/master/keps/sig-scheduling/20180409-scheduling-framework.md#custom-scheduler-plugins-out-of-tree). It selects a node directly.

To make multicluster-scheduler compatible with another custom scheduler, that scheduler would have to use the Scheduling Framework, be wrapped with multicluster-scheduler's plugins in addition to its own, and recompiled. [This may be too strong of a requirement?]

### Scale Limits

With this approach, every source pod goes through a scheduling cycle in each target cluster (as a candidate). If you were hitting single-cluster scheduler scale limits, e.g., in a high-throughput computing (HTC) use case, you'd now be hitting the same limits in all target clusters. In this scenario, the alternative presented below may be a better solution.

## Alternative

An alternative would be to have one virtual node per target cluster and let the source cluster's scheduler select a virtual node, i.e., cluster, informed by available resources and labels on virtual nodes. Target clusters would update corresponding virtual nodes in each cluster that they invite.

This two-step scheduling algorithm, while simple, may result in target pods not being schedulable after a cluster is selected, for multiple reasons:

- Bin-packing resolution loss. How should available resources be aggregated from a cluster of nodes and pods to a single virtual node? Using _sum_ as an aggregate, a cluster may have enough available resources summed over all of its nodes, but no node with enough resources on its own; or, using _max_ as an aggregate, a cluster may have enough CPU on one node and enough memory on another, but no node with enough of all resources on its own.

- Labeling. How should labels be aggregated from a cluster of nodes to a single virtual node? What about labels with identical keys but different values within a cluster? Finally, pod (anti-)affinities would have different meanings during the two scheduling steps, because of the confusion of topologies.
