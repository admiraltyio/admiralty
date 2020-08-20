# Installation

## CLI

The Admiralty command line interface (CLI) helps you sign up for an Admiralty Cloud account, register clusters, and exchange certificates between clusters in your account, or with clusters in other accounts. It is not strictly required, but it is the most secure and user-friendly way to authenticate cross-cluster control loops. It works with all Kubernetes v1.17+ clusters, including all cloud distributions, private clusters, etc.

1.  Download the CLI:

    ```shell script
    OS=linux # or darwin (i.e., OS X) or windows
    ARCH=amd64 # or arm64 (linux only)
    curl -Lo admiralty "https://artifacts.admiralty.io/admiralty-v0.10.0-$OS-$ARCH"
    chmod +x admiralty
    sudo mv admiralty /usr/local/bin
    ```

1.  Log in (sign up):

    ```shell script
    admiralty configure
    ```
   
    This will create a cluster, user, and context in your current kubeconfig (e.g., `~/.kube/config`, or whatever the `KUBECONFIG` environment variable points to), targeting the Admiralty Cloud API, which is a Kubernetes-like API. It will also set the current context to the new one, so don't forget to switch contexts to interact with your clusters, or switch kubeconfigs if using multiple files.

## Operator

1.  [Install Helm v3](https://helm.sh/docs/intro/install/) on your machine if not already installed, as it is the only supported way to install the Admiralty operator at the moment. Once installed, add the Admiralty chart repository:

    ```shell script
    helm repo add admiralty https://charts.admiralty.io
    helm repo update
    ```

    The Admiralty operator must be installed in all clusters that you want to connect. Repeat the following steps for each cluster:
    
1.  Set your current kubeconfig and context to target the cluster:

    ```shell script
    export KUBECONFIG=changeme # if using multiple kubeconfig files
    kubectl config use-context changeme # if using multiple contexts
    ```

1.  Install cert-manager v0.11+ if not already installed:

    ```shell script
    kubectl apply --validate=false -f https://github.com/jetstack/cert-manager/releases/download/v0.16.1/cert-manager.yaml
    ```
   
    Alternatively, you can [install it with Helm](https://cert-manager.io/docs/installation/kubernetes/#installing-with-helm).

1.  Choose a name for your cluster. It should be a [valid DNS label](https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#dns-label-names), and it should be unique within your Admiralty Cloud account:

    ```shell script
    CLUSTER_NAME=change-me
    ```

1.  Install the Admiralty operator with Helm v3:

    ```shell script
    kubectl create namespace admiralty
    helm install admiralty admiralty/admiralty \
        --namespace admiralty \
        --version 0.10.0 \
        --set accountName=$(admiralty get-account-name) \
        --set clusterName=$CLUSTER_NAME \
        --wait
    ```

1.  Register the cluster:

    ```shell script
    admiralty register-cluster
    ```
    
    Mainly, this informs Admiralty Cloud of the cluster's certificate authority (CA) certificate (the CA's private key doesn't leave the cluster, of course), to be distributed to trusted clusters (cf. [Authentication](authentication.md)), and opens the server side of a reverse tunnel to route Kubernetes API requests from other clusters to the cluster's kube-mtls-proxy.