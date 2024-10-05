---
title: Pod Configuration
custom_edit_url: https://github.com/admiraltyio/admiralty/edit/master/docs/user_guide/pod_configuration.md
---


## Label Prefixing
Admiralty prefixes delegate pod labels with `multicluster.admiralty.io/` in the target clusters. This behavior is useful in bursting cluster topologies, where the source cluster is also a target cluster, so as not to confuse controllers of proxy pods, e.g., replicasets. The behavior
can be overridden per pod through the `multicluster.admiralty.io/no-prefix-label-regexp` annotation. This is useful to 
support components that have functionality that relies on pod labels. For example, webhooks or monitoring resources.


:::tip
One use-case is to prevent prefixing the Kueue queue name label. This can be achieved through:
```
multicluster.admiralty.io/no-prefix-label-regexp="^kueue\.x-k8s\.io\/queue-name"
```
:::

