---
title: Quick Start
custom_edit_url: https://github.com/admiraltyio/admiralty/edit/master/docs/quick_start.md
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

This guide provides copy-and-paste instructions to try out the Admiralty open source cluster agent with or without Admiralty Cloud. We use [kind](https://kind.sigs.k8s.io/) (Kubernetes in Docker) to create Kubernetes clusters, but feel free to use something else—though don't just copy and paste instructions then.

## Example Use Case

We're going to model a centralized cluster topology made of a management cluster (named `cd`) where applications are deployed, and two workload clusters (named `us` and `eu`) where containers actually run. We'll deploy a batch job utilizing both workload clusters, and another targeting a specific region. If you're interested in other [topologies](./concepts/topologies.md) or other kinds of applications (e.g., micro-services), this guide is still helpful to get familiar with Admiralty in general.

<Tabs
defaultValue="global"
values={[
{label: 'Global batch', value: 'global'},
{label: 'Regional batch', value: 'regional'},
]}>
<TabItem value="global">

![](quick_start_data_plane_global_batch.svg)

</TabItem>
<TabItem value="regional">

![](quick_start_data_plane_regional_batch.svg)

</TabItem>
</Tabs>

## Prerequisites

1.  Install [Helm v3](https://helm.sh/docs/intro/install/) and [kind](https://kind.sigs.k8s.io/docs/user/quick-start#installation) if not already installed.

1.  We recommend you to use a separate kubeconfig for this exercise, so you can simply delete it when you're done:

    ```shell script
    export KUBECONFIG=kubeconfig-admiralty-getting-started
    ```

1.  Create three clusters (a management cluster named `cd` and two workload clusters named `us` and `eu`):

    ```shell script
    for CLUSTER_NAME in cd us eu
    do
      kind create cluster --name $CLUSTER_NAME
    done
    ```

1.  Label the workload cluster nodes as if they were in different regions (we'll use these labels as node selectors):

    ```shell script
    for CLUSTER_NAME in us eu
    do
      kubectl --context kind-$CLUSTER_NAME label nodes --all topology.kubernetes.io/region=$CLUSTER_NAME
    done
    ```

    :::tip
    Most cloud distributions of Kubernetes pre-label nodes with the names of their cloud regions.
    :::

1.  (optional speed-up) Pull images on your machine and load them into the kind clusters. Otherwise, each kind cluster would pull images, which could take three times as long.

    ```shell script
    images=(
      # cert-manager dependency
      quay.io/jetstack/cert-manager-controller:v0.16.1
      quay.io/jetstack/cert-manager-webhook:v0.16.1
      quay.io/jetstack/cert-manager-cainjector:v0.16.1
      # admiralty open source
      quay.io/admiralty/multicluster-scheduler-agent:0.12.0
      quay.io/admiralty/multicluster-scheduler-scheduler:0.12.0
      quay.io/admiralty/multicluster-scheduler-remove-finalizers:0.12.0
      quay.io/admiralty/multicluster-scheduler-restarter:0.12.0
      # admiralty cloud/enterprise
      quay.io/admiralty/admiralty-cloud-controller-manager:0.12.0
      quay.io/admiralty/kube-mtls-proxy:0.10.0
      quay.io/admiralty/kube-oidc-proxy:v0.3.0 # jetstack's image rebuilt for multiple architectures
    )
    for image in "${images[@]}"
    do
      docker pull $image
      for CLUSTER_NAME in cd us eu
      do
        kind load docker-image $image --name $CLUSTER_NAME
      done
    done
    ```

1.  Install [cert-manager](https://cert-manager.io/) in each cluster:

    ```shell script
    helm repo add jetstack https://charts.jetstack.io
    helm repo update

    for CLUSTER_NAME in cd us eu
    do
      kubectl --context kind-$CLUSTER_NAME create namespace cert-manager
      kubectl --context kind-$CLUSTER_NAME apply --validate=false -f https://github.com/jetstack/cert-manager/releases/download/v0.16.1/cert-manager.crds.yaml
      helm install cert-manager jetstack/cert-manager \
        --kube-context kind-$CLUSTER_NAME \
        --namespace cert-manager \
        --version v0.16.1 \
        --wait --debug
      # --wait to ensure release is ready before next steps
      # --debug to show progress, for lack of a better way,
      # as this may take a few minutes
    done
    ```

    :::note
    Admiralty Open Source uses cert-manager to generate a server certificate for its [mutating pod admission webhook](concepts/scheduling.md#proxy-pods). In addition, Admiralty Cloud and Admiralty Enterprise use cert-manager to generate server certificates for Kubernetes API authenticating proxies (mTLS for clusters, OIDC for users), and client certificates for cluster [identities](operator_guide/authentication.md#identities) (talking to the mTLS proxies of other clusters).
    :::

## Installation

Admiralty Cloud, its command line interface (CLI), and additional cluster-agent components complement the open-source cluster agent in useful ways. The CLI makes it easy to register clusters; Kubernetes custom resource definitions (CRDs) make it easy to connect them (with automatic certificate rotations), so you don't have to craft (and re-craft) cross-cluster kubeconfigs and think about routing and certificates.

Admiralty Cloud works with private clusters too. In this context, a private cluster is a cluster whose Kubernetes API isn't routable from another cluster. Cluster-to-cluster communications to private clusters transit through HTTPS/WebSocket/HTTPS tunnels exposed on the Admiralty Cloud API.

:::note Privacy Notice
We don't want to see your data. Admiralty Cloud cannot decrypt cluster-to-cluster communications, because private keys never leave the clusters. All clusters ever share with Admiralty Cloud are their CA certificates (public keys) to give to other clusters. Admiralty Cloud acts as a public key directory—"[Keybase](https://keybase.io/) for Kubernetes clusters" if you'd like.
:::

If you decide to use the open-source cluster agent only, no problem. There's no CLI nor cluster registration, but configuring cross-cluster authentication takes more care, and doesn't support private clusters. In production, you would have to rotate tokens manually.

<Tabs
groupId="oss-or-cloud"
defaultValue="cloud"
values={[
{label: 'Cloud/Enterprise', value: 'cloud'},
{label: 'Open Source', value: 'oss'},
]
}>
<TabItem value="cloud">

1.  Download the Admiralty CLI:

    <Tabs
    groupId="os"
    defaultValue="linux-amd64"
    values={[
    {label: 'Linux/amd64', value: 'linux-amd64'},
    {label: 'Mac', value: 'mac'},
    {label: 'Windows', value: 'windows'},
    {label: 'Linux/arm64', value: 'linux-arm64'},
    {label: 'Linux/ppc64le', value: 'linux-ppc64le'},
    {label: 'Linux/s390x', value: 'linux-s390x'},
    ]
    }>
    <TabItem value="linux-amd64">

    ```shell script
    curl -Lo admiralty "https://artifacts.admiralty.io/admiralty-v0.11.1-linux-amd64"
    chmod +x admiralty
    sudo mv admiralty /usr/local/bin
    ```

    </TabItem>
    <TabItem value="mac">

    ```shell script
    curl -Lo admiralty "https://artifacts.admiralty.io/admiralty-v0.11.1-darwin-amd64"
    chmod +x admiralty
    sudo mv admiralty /usr/local/bin
    ```

    </TabItem>
    <TabItem value="windows">

    ```shell script
    curl -Lo admiralty "https://artifacts.admiralty.io/admiralty-v0.11.1-windows-amd64"
    ```

    </TabItem>
    <TabItem value="linux-arm64">

    ```shell script
    curl -Lo admiralty "https://artifacts.admiralty.io/admiralty-v0.11.1-linux-arm64"
    chmod +x admiralty
    sudo mv admiralty /usr/local/bin
    ```

    </TabItem>
    <TabItem value="linux-ppc64le">

    ```shell script
    curl -Lo admiralty "https://artifacts.admiralty.io/admiralty-v0.11.1-linux-ppc64le"
    chmod +x admiralty
    sudo mv admiralty /usr/local/bin
    ```

    </TabItem>
    <TabItem value="linux-s390x">

    ```shell script
    curl -Lo admiralty "https://artifacts.admiralty.io/admiralty-v0.11.1-linux-s390x"
    chmod +x admiralty
    sudo mv admiralty /usr/local/bin
    ```

    </TabItem>
    </Tabs>

1.  Log in (sign up) to Admiralty Cloud:

    ```shell script
    admiralty configure
    ```

    :::note
    The `admiralty configure` command takes you through an OIDC log-in/sign-up flow, and eventually merges a context, cluster and user into the current kubeconfig (it also sets the current context!). The context is used to register clusters with the Admiralty Cloud API. Admiralty Cloud user tokens are saved in `~/.admiralty/tokens.json` (don't forget to run `admiralty logout` to delete this sensitive file if needed when you're done).
    :::

1.  Install Admiralty in each cluster:

    ```shell script
    helm repo add admiralty https://charts.admiralty.io
    helm repo update

    for CLUSTER_NAME in cd us eu
    do
      kubectl --context kind-$CLUSTER_NAME create namespace admiralty
      helm install admiralty admiralty/admiralty \
        --kube-context kind-$CLUSTER_NAME \
        --namespace admiralty \
        --version 0.12.0 \
        --set accountName=$(admiralty get-account-name) \
        --set clusterName=$CLUSTER_NAME \
        --wait --debug
      # --wait to ensure release is ready before next steps
      # --debug to show progress, for lack of a better way,
      # as this may take a few minutes
    done
    ```

1.  Register each cluster:

    ```shell script
    for CLUSTER_NAME in cd us eu
    do
      admiralty register-cluster --context kind-$CLUSTER_NAME
    done
    ```

</TabItem>
<TabItem value="oss">

Install Admiralty in each cluster:

```shell script
helm repo add admiralty https://charts.admiralty.io
helm repo update

for CLUSTER_NAME in cd us eu
do
  kubectl --context kind-$CLUSTER_NAME create namespace admiralty
  helm install admiralty admiralty/multicluster-scheduler \
    --kube-context kind-$CLUSTER_NAME \
    --namespace admiralty \
    --version 0.12.0 \
    --wait --debug
  # --wait to ensure release is ready before next steps
  # --debug to show progress, for lack of a better way,
  # as this may take a few minutes
done
```

</TabItem>
</Tabs>

## Configuration

### Cross-Cluster Authentication

<Tabs
groupId="oss-or-cloud"
defaultValue="cloud"
values={[
{label: 'Cloud/Enterprise', value: 'cloud'},
{label: 'Open Source', value: 'oss'},
]
}>
<TabItem value="cloud">

1.  In the management cluster, create a [Kubeconfig](operator_guide/authentication.md#kubeconfigs) for each workload cluster:

    ```shell script
    for CLUSTER_NAME in us eu
    do
      cat <<EOF | kubectl --context kind-cd apply -f -
    apiVersion: multicluster.admiralty.io/v1alpha1
    kind: Kubeconfig
    metadata:
      name: $CLUSTER_NAME
    spec:
      secretName: $CLUSTER_NAME
      cluster:
        admiraltyReference:
          clusterName: $CLUSTER_NAME
    EOF
    done
    ```

1.  In each workload cluster, create a [TrustedIdentityProvider](operator_guide/authentication.md#trusted-identity-providers) for the management cluster:

    ```shell script
    for CLUSTER_NAME in us eu
    do
      cat <<EOF | kubectl --context kind-$CLUSTER_NAME apply -f -
    apiVersion: multicluster.admiralty.io/v1alpha1
    kind: TrustedIdentityProvider
    metadata:
      name: cd
    spec:
      prefix: "spiffe://cd/"
      admiraltyReference:
        clusterName: cd
    EOF
    done
    ```

</TabItem>
<TabItem value="oss">

1.  Install [jq](https://stedolan.github.io/jq/download/), the command-line JSON processor, if not already installed.

1.  For each workload cluster,

    1. create a Kubernetes service account in the workload cluster for the management cluster,
    1. extract its default token,
    1. get a Kubernetes API address that is routable from the management cluster—here, the IP address of the kind workload cluster's only (master) node container in your machine's shared Docker network,
    1. prepare a kubeconfig using the token and address found above, and the server certificate from your kubeconfig (luckily also valid for this address, not just the address in your kubeconfig),
    1. save the prepared kubeconfig in a secret in the management cluster:

    ```bash
    for CLUSTER_NAME in us eu
    do
      # i.
      kubectl --context kind-$CLUSTER_NAME create serviceaccount cd

      # ii.
      SECRET_NAME=$(kubectl --context kind-$CLUSTER_NAME get serviceaccount cd \
        --output json | \
        jq -r .secrets[0].name)
      TOKEN=$(kubectl --context kind-$CLUSTER_NAME get secret $SECRET_NAME \
        --output json | \
        jq -r .data.token | \
        base64 --decode)

      # iii.
      IP=$(docker inspect $CLUSTER_NAME-control-plane \
        --format "{{ .NetworkSettings.Networks.kind.IPAddress }}")

      # iv.
      CONFIG=$(kubectl --context kind-$CLUSTER_NAME config view \
        --minify --raw --output json | \
        jq '.users[0].user={token:"'$TOKEN'"} | .clusters[0].cluster.server="https://'$IP':6443"')

      # v.
      kubectl --context kind-cd create secret generic $CLUSTER_NAME \
        --from-literal=config="$CONFIG"
    done
    ```

    :::note Security Notice
    Kubernetes service account tokens exposed as secrets are valid forever, or until those secrets are deleted. A leak may go undetected indefinitely. If you use Kubernetes service account tokens as a cross-cluster authentication method in production, we recommend rotating the tokens as often as practical. However, [there are other methods](concepts/authentication.md), including using Admiralty Cloud.
    :::

    :::note Other Platforms
    If you're not using kind, your mileage may vary. The Kubernetes API address in your kubeconfig may or may not be routable from other clusters. If not, the server certificate in your kubeconfig may or may not be valid for the routable address that you'll find instead.
    :::

</TabItem>
</Tabs>

### Multi-Cluster Scheduling

1.  In the management cluster, create a [Target](operator_guide/scheduling.md#targets-and-cluster-targets) for each workload cluster:

    ```shell script
    for CLUSTER_NAME in us eu
    do
      cat <<EOF | kubectl --context kind-cd apply -f -
    apiVersion: multicluster.admiralty.io/v1alpha1
    kind: Target
    metadata:
      name: $CLUSTER_NAME
    spec:
      kubeconfigSecret:
        name: $CLUSTER_NAME
    EOF
    done
    ```

1.  In the workload clusters, create a [Source](operator_guide/scheduling.md#sources-and-cluster-sources) for the management cluster:

    <Tabs
    groupId="oss-or-cloud"
    defaultValue="cloud"
    values={[
    {label: 'Cloud/Enterprise', value: 'cloud'},
    {label: 'Open Source', value: 'oss'},
    ]
    }>
    <TabItem value="cloud">

    ```shell script
    for CLUSTER_NAME in us eu
    do
      cat <<EOF | kubectl --context kind-$CLUSTER_NAME apply -f -
    apiVersion: multicluster.admiralty.io/v1alpha1
    kind: Source
    metadata:
      name: cd
    spec:
      userName: spiffe://cd/ns/default/id/default
    EOF
    done
    ```

    </TabItem>
    <TabItem value="oss">

    ```shell script
    for CLUSTER_NAME in us eu
    do
      cat <<EOF | kubectl --context kind-$CLUSTER_NAME apply -f -
    apiVersion: multicluster.admiralty.io/v1alpha1
    kind: Source
    metadata:
      name: cd
    spec:
      serviceAccountName: cd
    EOF
    done
    ```

    </TabItem>
    </Tabs>

## Demo

1.  Check that virtual nodes have been created in the management cluster to represent workload clusters:

    ```shell script
    kubectl --context kind-cd get nodes --watch
    # --watch until virtual nodes are created,
    # this may take a few minutes, then control-C
    ```

1.  Label the `default` namespace in the management cluster to enable multi-cluster scheduling at the namespace level:

    ```shell script
    kubectl --context kind-cd label ns default multicluster-scheduler=enabled
    ```

1.  Create Kubernetes Jobs in the management cluster, utilizing all workload clusters (multi-cluster scheduling is enabled at the pod level with the `multicluster.admiralty.io/elect` annotation):

    ```shell script {12}
    for i in $(seq 1 10)
    do
      cat <<EOF | kubectl --context kind-cd apply -f -
    apiVersion: batch/v1
    kind: Job
    metadata:
      name: global-$i
    spec:
      template:
        metadata:
          annotations:
            multicluster.admiralty.io/elect: ""
        spec:
          containers:
          - name: c
            image: busybox
            command: ["sh", "-c", "echo Processing item $i && sleep 5"]
            resources:
              requests:
                cpu: 100m
          restartPolicy: Never
    EOF
    done
    ```

1.  Check that [proxy pods](concepts/scheduling.md#proxy-pods) for this job have been created in the management cluster, "running" on virtual nodes, and [delegate pods](concepts/scheduling.md#delegate-pods) have been created in the workload clusters, _actually_ running their containers on _real_ nodes:

    ```shell script
    while true
    do
      clear
      for CLUSTER_NAME in cd us eu
      do
        kubectl --context kind-$CLUSTER_NAME get pods -o wide
      done
      sleep 2
    done
    # control-C when all pods have Completed
    ```

1.  Create Kubernetes Jobs in the management cluster, targeting a specific region with a node selector:

    ```shell script {14-15}
    for i in $(seq 1 10)
    do
      cat <<EOF | kubectl --context kind-cd apply -f -
    apiVersion: batch/v1
    kind: Job
    metadata:
      name: eu-$i
    spec:
      template:
        metadata:
          annotations:
            multicluster.admiralty.io/elect: ""
        spec:
          nodeSelector:
            topology.kubernetes.io/region: eu
          containers:
          - name: c
            image: busybox
            command: ["sh", "-c", "echo Processing item $i && sleep 5"]
            resources:
              requests:
                cpu: 100m
          restartPolicy: Never
    EOF
    done
    ```

1.  Check that proxy pods for this job have been created in the management cluster, and delegate pods have been created in the `eu` cluster only:

    ```shell script
    while true
    do
      clear
      for CLUSTER_NAME in cd us eu
      do
        kubectl --context kind-$CLUSTER_NAME get pods -o wide
      done
      sleep 2
    done
    # control-C when all pods have Completed
    ```

    You may observe transient pending [candidate pods](concepts/scheduling.md#candidate-pods) in the `us` cluster.

## Cleanup

```shell script
for CLUSTER_NAME in cd us eu
do
  kind delete cluster --name $CLUSTER_NAME
done
rm kubeconfig-admiralty-getting-started
```
