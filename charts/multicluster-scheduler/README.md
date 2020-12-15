# Multicluster-Scheduler Helm Chart

Multicluster-Scheduler is a system of Kubernetes controllers that intelligently schedules workloads across clusters. This document details the parameters of the chart, and discusses our use of a Helm post-delete hook. For general information about the project and installation instructions, see the [main README](../../README.md).

> Note: This chart was built for Helm v3.

> Note: multicluster-scheduler doesn't support clusters without RBAC. There's no `rbac.create` parameter.

## Post-Delete Hook

Multicluster-scheduler uses [finalizers](https://kubernetes.io/docs/tasks/access-kubernetes-api/custom-resources/custom-resource-definitions/#finalizers) for [cross-cluster garbage collection](https://twitter.com/adrienjt/status/1199467878015066112). In particular, it adds finalizers to proxy pods, global services, and config maps and secrets mounted by multi-cluster pods. The finalizers block the deletion of those objects until multicluster-scheduler deletes their delegates in other clusters. If multicluster-scheduler stopped running, and those finalizers weren't removed, object deletions would be blocked indefinitely. Therefore, when multicluster-scheduler is uninstalled, a Kubernetes job will run as a [Helm post-delete hook](https://helm.sh/docs/topics/charts_hooks/) to clean up the finalizers.

## Parameters

| Key | Type | Default | Comment |
| --- | --- | --- | --- |
| sourceController.enabled | boolean | `true` | disable to configure source RBAC yourself |
| nameOverride | string | `""` | Override chart name in object names and labels |
| fullnameOverride | string | `""` | Override chart and release names in object names |
| imagePullSecretName | string | `""` |  |
| controllerManager.image.repository | string | `"quay.io/admiralty/multicluster-scheduler-agent"` |  |
| controllerManager.image.tag | string | `"0.13.2"` |  |
| controllerManager.image.pullPolicy | string | `"IfNotPresent"` |  |
| controllerManager.resources | object | `{}` |  |
| controllerManager.nodeSelector | object | `{}` |  |
| controllerManager.securityContext | object | `{}` |  |
| controllerManager.affinity | object | `{}` |  |
| controllerManager.tolerations | array | `[]` |  |
| scheduler.image.repository | string | `"quay.io/admiralty/multicluster-scheduler-scheduler"` |  |
| scheduler.image.tag | string | `"0.13.2"` |  |
| scheduler.image.pullPolicy | string | `"IfNotPresent"` |  |
| scheduler.resources | object | `{}` |  |
| scheduler.nodeSelector | object | `{}` |  |
| scheduler.securityContext | object | `{}` |  |
| scheduler.affinity | object | `{}` |  |
| scheduler.tolerations | array | `[]` |  |
| postDeleteJob.image.repository | string | `"quay.io/admiralty/multicluster-scheduler-remove-finalizers"` |  |
| postDeleteJob.image.tag | string | `"0.13.2"` |  |
| postDeleteJob.image.pullPolicy | string | `"IfNotPresent"` |  |
| postDeleteJob.resources | object | `{}` |  |
| postDeleteJob.nodeSelector | object | `{}` |  |
| postDeleteJob.securityContext | object | `{}` |  |
| postDeleteJob.affinity | object | `{}` |  |
| postDeleteJob.tolerations | array | `[]` |  |
| restarter.image.repository | string | `"quay.io/admiralty/multicluster-scheduler-remove-finalizers"` |  |
| restarter.image.tag | string | `"0.13.2"` |  |
| restarter.image.pullPolicy | string | `"IfNotPresent"` |  |
| restarter.resources | object | `{}` |  |
| restarter.nodeSelector | object | `{}` |  |
| restarter.securityContext | object | `{}` |  |
| restarter.affinity | object | `{}` |  |
| restarter.tolerations | array | `[]` |  |
