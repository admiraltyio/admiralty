---
title: Installation
custom_edit_url: https://github.com/admiraltyio/admiralty/edit/master/docs/operator_guide/installation.md
---



1.  [Install Helm v3](https://helm.sh/docs/intro/install/) on your machine if not already installed, as it is the only supported way to install the Admiralty agent at the moment.

    The Admiralty agent must be installed in all clusters that you want to connect. Repeat the following steps for each cluster:

1.  Set your current kubeconfig and context to target the cluster:

    ```shell script
    export KUBECONFIG=changeme # if using multiple kubeconfig files
    kubectl config use-context changeme # if using multiple contexts
    ```

1.  Refer to the [cert-manager documentation](https://cert-manager.io/docs/installation/kubernetes/) to install version 1.0+, if not already installed.

1.  Install the Admiralty agent with Helm v3:

    ```shell script
    helm install admiralty oci://public.ecr.aws/admiralty/admiralty \
        --namespace admiralty --create-namespace \
        --version 0.16.0 \
        --wait
    ```
