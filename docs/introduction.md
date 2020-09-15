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
1. Pod dependencies (configmaps and secrets only, for now) "follow" delegate pods, i.e., they are copied as needed to target clusters.
1. A feedback loop updates the statuses and annotations of the proxy pods to reflect the statuses and annotations of the delegate pods.
1. Services that target proxy pods are rerouted to their delegates, replicated across clusters, and annotated with `io.cilium/global-service=true` to be [load-balanced across a Cilium cluster mesh](http://docs.cilium.io/en/stable/gettingstarted/clustermesh/#load-balancing-with-global-services), if installed. (Other integrations are possible, e.g., with [Linkerd](https://linkerd.io/2/features/multicluster/) or [Istio](https://istio.io/latest/docs/ops/deployment/deployment-models/#multiple-clusters); please [tell us about your network setup](https://admiralty.io/contact).)
