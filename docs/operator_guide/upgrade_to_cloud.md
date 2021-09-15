---
title: Upgrading from Open Source to Cloud/Enterprise
custom_edit_url: https://github.com/admiraltyio/admiralty/edit/master/docs/operator_guide/oss_to_cloud_upgrade.md
---

If you want to upgrade a cluster that currently runs the Admiralty's open source cluster agent to Admiralty Cloud/Enterprise, e.g., to register it, connect it more easily with other clusters, generate OIDC kubeconfigs for users, and/or orchestrate global DNS load balancing, follow this guide.

The Admiralty Cloud/Enterprise Helm chart includes the open source cluster agent chart (named multicluster-scheduler) as a subchart, making it possible to upgrade without having to uninstall and reinstall.

1.  We're assuming that you installed Admiralty in the "admiralty" namespace and named the Helm release "admiralty" (you may have named it "multicluster-scheduler" if you've been with us for a long time 😉).

    ```shell
    RELEASE_NAME=admiralty # or is it "multicluster-scheduler"?
    NAMESPACE=admiralty
    ```

    :::tip
    Don't remember?

    ```shell
    helm list --all-namespaces
    ```

    :::

1.  If you used custom values, move them under the `multicluster-scheduler` key in a new values file, so that

    ```yaml
    key: value
    ```

    becomes

    ```yaml
    multicluster-scheduler:
      key: value
    ```

    :::tip
    If you're unsure whether you used custom values, e.g., because you set them on the command line rather than from a file, or you lost the file, you can get them from Helm:

    ```shell
    helm get values $RELEASE_NAME --namespace $NAMESPACE --output yaml > values.yaml
    ```

    :::

1.  Install Admiralty Cloud/Enterprise custom resource definitions (CRDs), because Helm [doesn't support](https://github.com/helm/helm/issues/6581) adding CRDs as part of upgrades:

    ```shell
    kubectl apply -f https://artifacts.admiralty.io/admiralty-v0.14.0.crds.yaml
    ```

1.  [Download the Admiralty CLI](installation.md#command-line-interface) if not already installed.

1.  Choose a name for your cluster, which will be useful, e.g., for referring to it from other clusters:

    ```shell
    CLUSTER_NAME=change-me
    ```

1.  Upgrade the Helm release:

    ```shell
    helm repo add admiralty https://charts.admiralty.io
    helm repo update
    helm upgrade $RELEASE_NAME admiralty/admiralty \
      --namespace $NAMESPACE \
      --version 0.14.0 \
      -f values.yaml \
      --set accountName=$(admiralty get-account-name) \
      --set clusterName=$CLUSTER_NAME
    ```

    Omit `-f values.yaml` if you didn't use custom values; and if you did, you may want to include `accountName` and `clusterName` in that file for future reference.

1.  Register the cluster:

    ```shell
    admiralty register-cluster
    ```

    :::caution
    `admiralty register-cluster` gets the name of the cluster CA Secret—where the public key to register is stored, alongside the private one—from the Admiralty ConfigMap. If the release is installed in the "admiralty" namespace and named "admiralty", as recommended, the ConfigMap is also named "admiralty", and created in that namespace. That's what `admiralty register-cluster` assumes by default. However, if the release name isn't "admiralty", the ConfigMap is named `$RELEASE_NAME-admiralty`; the CLI needs to know.

    ```shell
    if [ $RELEASE_NAME = admiralty ]; then
      CM_NAME=admiralty
    else
      CM_NAME=$RELEASE_NAME-admiralty
    fi

    admiralty register-cluster \
      --admiralty-config-map-name $CM_NAME \
      --admiralty-config-map-namespace $NAMESPACE
    ```

    :::
