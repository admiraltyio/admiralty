---
title: Installation
custom_edit_url: https://github.com/admiraltyio/admiralty/edit/master/docs/operator_guide/installation.md
---



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

1.  Install the Admiralty agent with Helm v3:

    ```shell script
    kubectl create namespace admiralty
    helm install admiralty admiralty/multicluster-scheduler \
        --namespace admiralty \
        --version 0.14.1 \
        --wait
    ```
