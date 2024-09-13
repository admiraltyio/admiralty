---
title: Pod Configuration
custom_edit_url: https://github.com/admiraltyio/admiralty/edit/master/docs/user_guide/pod_configuration.md
---


## Label Prefixing
Admiralty appends a prefix to the delegate pod labels `multicluster.admiralty.io/` in the target clusters. This behavior
can be overridden per pod through the `multicluster.admiralty.io/no-prefix-label-regexp` annotation. This is useful to 
support components that have functionality that relies on pod labels. For example, webhooks or monitoring resources.


:::tip
One use-case is to prevent prefixing the Kueue queue name label. This can be achieved through:
```
multicluster.admiralty.io/no-prefix-label-regexp="^kueue\.x-k8s\.io\/queue-name"
```
:::

