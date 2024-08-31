# Admiralty Helm Chart

Admiralty is a system of Kubernetes controllers that intelligently schedules workloads across clusters. This document details the parameters of the chart, and discusses our use of a Helm post-delete hook. For general information about the project and installation instructions, see the [main README](../../README.md).

> Note: This chart was built for Helm v3.

> Note: Admiralty doesn't support clusters without RBAC. There's no `rbac.create` parameter.

## Post-Delete Hook

Admiralty uses [finalizers](https://kubernetes.io/docs/tasks/access-kubernetes-api/custom-resources/custom-resource-definitions/#finalizers) for [cross-cluster garbage collection](https://twitter.com/adrienjt/status/1199467878015066112). In particular, it adds finalizers to proxy pods, global services, and config maps and secrets mounted by multi-cluster pods. The finalizers block the deletion of those objects until Admiralty deletes their delegates in other clusters. If Admiralty stopped running, and those finalizers weren't removed, object deletions would be blocked indefinitely. Therefore, when Admiralty is uninstalled, a Kubernetes job will run as a [Helm post-delete hook](https://helm.sh/docs/topics/charts_hooks/) to clean up the finalizers.

## Parameters

| Key                                | Type    | Default                                                  | Comment                                          |
|------------------------------------|---------|----------------------------------------------------------|--------------------------------------------------|
| sourceController.enabled           | boolean | `true`                                                   | disable to configure source RBAC yourself        |
| nameOverride                       | string  | `""`                                                     | Override chart name in object names and labels   |
| fullnameOverride                   | string  | `""`                                                     | Override chart and release names in object names |
| imagePullSecretName                | string  | `""`                                                     |                                                  |
| controllerManager.logLevel         | integer | `2`                                                      | log verbosity level                              |
| controllerManager.replicas         | integer | `2`                                                      |                                                  |
| controllerManager.image.repository | string  | `"public.ecr.aws/admiralty/admiralty-agent"`             |                                                  |
| controllerManager.image.tag        | string  | `"0.16.0"`                                               |                                                  |
| controllerManager.image.pullPolicy | string  | `"IfNotPresent"`                                         |                                                  |
| controllerManager.resources        | object  | `{}`                                                     |                                                  |
| controllerManager.nodeSelector     | object  | `{}`                                                     |                                                  |
| controllerManager.securityContext  | object  | `{}`                                                     |                                                  |
| controllerManager.affinity         | object  | `{}`                                                     |                                                  |
| controllerManager.tolerations      | array   | `[]`                                                     |                                                  |
| scheduler.replicas                 | integer | `2`                                                      |                                                  |
| scheduler.image.repository         | string  | `"public.ecr.aws/admiralty/admiralty-scheduler"`         |                                                  |
| scheduler.image.tag                | string  | `"0.16.0"`                                               |                                                  |
| scheduler.image.pullPolicy         | string  | `"IfNotPresent"`                                         |                                                  |
| scheduler.resources                | object  | `{}`                                                     |                                                  |
| scheduler.nodeSelector             | object  | `{}`                                                     |                                                  |
| scheduler.securityContext          | object  | `{}`                                                     |                                                  |
| scheduler.affinity                 | object  | `{}`                                                     |                                                  |
| scheduler.tolerations              | array   | `[]`                                                     |                                                  |
| scheduler.proxy.logLevel           | integer | `2`                                                      | log verbosity level                              |
| scheduler.proxy.pluginConfig       | array   | `[]`                                                     | set of custom plugin arguments for each plugin   |
| scheduler.candidate.logLevel       | integer | `2`                                                      | log verbosity level                              |
| scheduler.proxy.pluginConfig       | array   | `[]`                                                     | set of custom plugin arguments for each plugin   |
| postDeleteJob.image.repository     | string  | `"public.ecr.aws/admiralty/admiralty-remove-finalizers"` |                                                  |
| postDeleteJob.image.tag            | string  | `"0.16.0"`                                               |                                                  |
| postDeleteJob.image.pullPolicy     | string  | `"IfNotPresent"`                                         |                                                  |
| postDeleteJob.resources            | object  | `{}`                                                     |                                                  |
| postDeleteJob.nodeSelector         | object  | `{}`                                                     |                                                  |
| postDeleteJob.securityContext      | object  | `{}`                                                     |                                                  |
| postDeleteJob.affinity             | object  | `{}`                                                     |                                                  |
| postDeleteJob.tolerations          | array   | `[]`                                                     |                                                  |
| restarter.replicas                 | integer | `2`                                                      |                                                  |
| restarter.image.repository         | string  | `"public.ecr.aws/admiralty/admiralty-remove-finalizers"` |                                                  |
| restarter.image.tag                | string  | `"0.16.0"`                                               |                                                  |
| restarter.image.pullPolicy         | string  | `"IfNotPresent"`                                         |                                                  |
| restarter.resources                | object  | `{}`                                                     |                                                  |
| restarter.nodeSelector             | object  | `{}`                                                     |                                                  |
| restarter.securityContext          | object  | `{}`                                                     |                                                  |
| restarter.affinity                 | object  | `{}`                                                     |                                                  |
| restarter.tolerations              | array   | `[]`                                                     |                                                  |
| webhook.reinvocationPolicy         | string  | `"Never"`                                                |                                                  |
