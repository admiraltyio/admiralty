---
title: Installation
custom_edit_url: https://github.com/admiraltyio/admiralty/edit/master/docs/operator_guide/installation.md
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

## Command Line Interface

<Tabs
groupId="oss-or-cloud"
defaultValue="cloud"
values={[
{label: 'Cloud/Enterprise', value: 'cloud'},
{label: 'Open Source', value: 'oss'},
]
}>
<TabItem value="cloud">

The Admiralty command line interface (CLI) helps you sign up for an Admiralty Cloud account, register clusters, and exchange certificates between clusters in your account, or with clusters in other accounts. It is not strictly required, but it is the most secure and user-friendly way to authenticate cross-cluster control loops. It works with all Kubernetes v1.17+ clusters, including all cloud distributions, private clusters, etc.

1.  Download the CLI:

    ```shell script
    OS=linux # or darwin (i.e., OS X) or windows
    ARCH=amd64 # or, for linux, any of arm64, ppc64le, s390x
    curl -Lo admiralty "https://artifacts.admiralty.io/admiralty-v0.13.1-$OS-$ARCH"
    chmod +x admiralty
    sudo mv admiralty /usr/local/bin
    ```

1.  Log in (sign up):

    ```shell script
    admiralty configure
    ```

</TabItem>
<TabItem value="oss">

The Admiralty command line interface (CLI) isn't required if you don't use Admiralty Cloud features.

</TabItem>
</Tabs>

## Agent

1.  [Install Helm v3](https://helm.sh/docs/intro/install/) on your machine if not already installed, as it is the only supported way to install the Admiralty agent at the moment. Once installed, add the Admiralty chart repository:

    ```shell script
    helm repo add admiralty https://charts.admiralty.io
    helm repo update
    ```

    The Admiralty agent must be installed in all clusters that you want to connect. Repeat the following steps for each cluster:

1.  Set your current kubeconfig and context to target the cluster:

    ```shell script
    export KUBECONFIG=changeme # if using multiple kubeconfig files
    kubectl config use-context changeme # if using multiple contexts
    ```

1.  Refer to the [cert-manager documentation](https://cert-manager.io/docs/installation/kubernetes/) to install version 0.11+, if not already installed.

<Tabs
groupId="oss-or-cloud"
defaultValue="cloud"
values={[
{label: 'Cloud/Enterprise', value: 'cloud'},
{label: 'Open Source', value: 'oss'},
]
}>
<TabItem value="cloud">

4.  Choose a name for your cluster. It should be a [valid DNS label](https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-label-names), and it should be unique within your Admiralty Cloud account:

    ```shell script
    CLUSTER_NAME=change-me
    ```

1.  Install the Admiralty agent with Helm v3:

    ```shell script
    kubectl create namespace admiralty
    helm install admiralty admiralty/admiralty \
        --namespace admiralty \
        --version 0.13.1 \
        --set accountName=$(admiralty get-account-name) \
        --set clusterName=$CLUSTER_NAME \
        --wait
    ```

1.  Register the cluster:

    ```shell script
    admiralty register-cluster
    ```

    Mainly, this informs Admiralty Cloud of the cluster's certificate authority (CA) certificate (the CA's private key doesn't leave the cluster, of course), to be distributed to trusted clusters (cf. [Authentication](authentication.md)), and opens the server side of a reverse tunnel to route Kubernetes API requests from other clusters to the cluster's kube-mtls-proxy.

</TabItem>
<TabItem value="oss">

4.  Install the Admiralty agent with Helm v3:

    ```shell script
    kubectl create namespace admiralty
    helm install admiralty admiralty/multicluster-scheduler \
        --namespace admiralty \
        --version 0.13.1 \
        --wait
    ```

</TabItem>
</Tabs>
