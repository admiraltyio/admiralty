---
title: Introduction
slug: /
custom_edit_url: https://github.com/admiraltyio/admiralty/edit/master/docs/introduction.md
---

Admiralty is a system of Kubernetes controllers that intelligently schedules workloads across clusters. It is simple to use and simple to integrate with other tools. It enables many multi-cluster, multi-region, multi-cloud, and hybrid (simply put, global computing) use cases:

<ul style={{columns: 2}}>
<li>high availability,</li>
<li>active-active disaster recovery,</li>
<li>dynamic content delivery networks (dCDNs),</li>
<li>distributed workflows,</li>
<li>edge computing, Internet of Things (IoT), 5G,</li>
<li>central access control and auditing,</li>
<li>blue/green cluster upgrades,</li>
<li>cluster abstraction (clusters as cattle),</li>
<li>resource federation (including global research platforms),</li>
<li>cloud bursting,</li>
<li>cloud arbitrage...</li>
</ul>

In a nutshell, here's how Admiralty works:

1. Install Admiralty in each cluster that you want to federate. Configure clusters as sources and/or targets to build a centralized or decentralized topology.
1. Annotate any pod or pod template (e.g., of a Deployment, Job, or [Argo](https://argoproj.github.io/projects/argo) Workflow, among others) in any source cluster with `multicluster.admiralty.io/elect=""`.
1. Admiralty mutates the elected pods into _proxy pods_ scheduled on [virtual-kubelet](https://virtual-kubelet.io/) nodes representing target clusters, and creates _delegate pods_ in the remote clusters (actually running the containers).
1. Pod dependencies (config maps and secrets) and dependents (services and ingresses) "follow" delegate pods, i.e., they are copied as needed to target clusters.
1. A feedback loop updates the statuses and annotations of the proxy pods to reflect the statuses and annotations of the delegate pods.
1. `kubectl logs` and `kubectl exec` work as expected.
1. Integrate with Admiralty Cloud/Enterprise, [Cilium](https://cilium.io/blog/2019/03/12/clustermesh/) and other third-party solutions to enable north-south and east-west networking across clusters.

:::note Open Source and Admiralty Cloud/Enterprise
This documentation covers both the Admiralty open source cluster agent and Admiralty Cloud/Enterprise. Features only available with Admiralty Cloud/Enterprise are clearly marked; in that case, as much as possible, open source and commercial third-party alternatives are discussed.
:::
