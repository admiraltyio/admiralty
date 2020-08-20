# Getting Started

In this guide, we use [kind](https://kind.sigs.k8s.io/) (Kubernetes in Docker) to create Kubernetes clusters, and Admiralty Cloud to facilitate cluster connections. We're going to model a centralized cluster topology: a management cluster where applications are deployed, and multiple workload clusters where containers actually run. If you're interested in other topologies, this guide is still helpful to get familiar with Admiralty in general.

1.  Install [Helm v3]((https://helm.sh/docs/intro/install/)) and [kind](https://kind.sigs.k8s.io/docs/user/quick-start#installation) if not already installed.

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
        --wait
    done
    ```

1.  Download the Admiralty CLI:

    ```shell script
    OS=linux # or darwin (i.e., OS X) or windows
    ARCH=amd64 # or arm64 (linux only)
    curl -Lo admiralty "https://artifacts.admiralty.io/admiralty-v0.10.0-$OS-$ARCH"
    chmod +x admiralty
    sudo mv admiralty /usr/local/bin
    ```

1.  Log in (sign up) to Admiralty Cloud:

    ```shell script
    admiralty configure
    ```

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
        --version 0.10.0 \
        --set accountName=$(admiralty get-account-name) \
        --set clusterName=$CLUSTER_NAME \
        --wait
    done
    ```

1.  Register each cluster:

    ```shell script
    for CLUSTER_NAME in cd us eu
    do
      admiralty register-cluster --context kind-$CLUSTER_NAME
    done
    ```

1.  In the management cluster, create an [authentication Kubeconfig](operator_guide/authentication.md#kubeconfigs) and a [scheduling Target](operator_guide/scheduling.md#targets-and-cluster-targets) for each workload cluster:

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
    ---
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

1.  In the workload clusters, create an [authentication TrustedIdentityProvider](operator_guide/authentication.md#trusted-identity-providers) and a [scheduling Source](operator_guide/scheduling.md#sources-and-cluster-sources) for the management cluster:

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
    ---
    apiVersion: multicluster.admiralty.io/v1alpha1
    kind: Source
    metadata:
      name: cd
    spec:
      userName: spiffe://cd/ns/default/id/default
    EOF
    done
    ```

1.  Check that virtual nodes have been created in the management cluster to represent workload clusters:

    ```shell script
    kubectl --context kind-cd get nodes
    ```

1.  Label the `default` namespace in the management cluster to enable multi-cluster scheduling at the namespace level:

    ```shell script
    kubectl --context kind-cd label ns default multicluster-scheduler=enabled
    ```

1.  Create Kubernetes Jobs in the management cluster, utilizing all workload clusters (multi-cluster scheduling is enabled at the pod level with the `multicluster.admiralty.io/elect` annotation):

    ```shell script
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

1.  Check that proxy pods for this job have been created in the management cluster, "running" on virtual nodes, and delegate pods have been created in the workload clusters, _actually_ running their containers on _real_ nodes:

    ```shell script
    for CLUSTER_NAME in cd us eu
    do
      kubectl --context kind-$CLUSTER_NAME get pods -o wide
    done
    ```

1.  Create Kubernetes Jobs in the management cluster, targeting a specific region with a node selector:

    ```shell script
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

1.  Check that proxy pods for this job have been created in the management cluster, and delegate pods have been created in the `eu` cluster:

    ```shell script
    for CLUSTER_NAME in cd us eu
    do
      kubectl --context kind-$CLUSTER_NAME get pods -o wide
    done
    ```
    
    You might observe transient pending _candidate_ pods in the `us` cluster.

1.  Clean up:

    ```shell script
    for CLUSTER_NAME in cd us eu
    do
      kind delete cluster --name $CLUSTER_NAME
    done
    admiralty logout
    rm kubeconfig-admiralty-getting-started
    ```
