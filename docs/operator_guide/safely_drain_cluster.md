---
title: Safely Drain a Cluster
custom_edit_url: https://github.com/admiraltyio/admiralty/edit/master/docs/operator_guide/safely_drain_cluster.md
---

Just like you can [safely drain a node](https://kubernetes.io/docs/tasks/administer-cluster/safely-drain-node/) with Kubernetes, you can safely drain a cluster with Admiralty. The operation even respects [PodDisruptionBudgets](https://kubernetes.io/docs/concepts/workloads/pods/disruptions/). This is possible because target clusters are represented by virtual nodes in source clusters. This feature is particularly useful in a central control plane topology to perform blue/green cluster upgrades.

Given a cluster connection between a source cluster and a target named `c2` in the `default` namespace, there should be a virtual node named `admiralty-default-c2` in the source cluster. To evict all proxy pods running on that node, hence delete all corresponding delegate pods in that target cluster, run:

```sh
kubectl drain admiralty-default-c2
```

Note that delegate pods in that target cluster owned by other source clusters, if any, are not affected.

`kubectl drain` also cordons the virtual node, so that it becomes unschedulable (by the way, `kubectl cordon` works too!). If the evicted proxy pods are owned by other objects, e.g., ReplicaSets, they are replaced by new ones that are scheduled to available virtual nodes.

If you bring the target cluster back online, you need to run

```sh
kubectl uncordon admiralty-default-c2
```

in the source cluster afterward to tell the Admiralty proxy scheduler that it can resume scheduling new proxy pods onto the virtual node.
