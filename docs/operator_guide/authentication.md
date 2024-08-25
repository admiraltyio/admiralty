---
title: Configuring Authentication
custom_edit_url: https://github.com/admiraltyio/admiralty/edit/master/docs/operator_guide/authentication.md
---

:::info Admiralty Cloud/Enterprise Only
This section discusses the cluster identity federation feature available in Admiralty Cloud/Enterprise. With Admiralty Open Source, you're in charge of creating kubeconfig Secrets. For example, you can use Service Account tokens from target clusters; [the Quick Start guide a provides a helper script for that](../quick_start#cross-cluster-authentication). For a discussion of several methods, [please refer to the related Concepts section](../concepts/authentication.md).
:::

If your cluster needs to call the Kubernetes API of another cluster, e.g., to send pods to it, you should at least create a [Kubeconfig](#kubeconfigs) object.

If your cluster's Kubernetes API needs to be called by another cluster, e.g., to receive pods from it, you should create a [TrustedIdentityProvider](#trusted-identity-providers) object.

## Kubeconfigs

The Admiralty agent generates kubeconfig secrets from Kubeconfig objects. Those kubeconfig secrets can be used by Admiralty's cross-cluster controllers (and by third-party cross-cluster controllers). A minimal Kubeconfig object looks like this:

```yaml
apiVersion: multicluster.admiralty.io/v1alpha1
kind: Kubeconfig
metadata:
  name: target-cluster
  namespace: namespace-a
spec:
  secretName: target-cluster-kubeconfig
  cluster:
    admiraltyReference:
      accountName: user-abcdef # defaults to the cluster's account name
      clusterName: target-cluster
```

In this example, the resulting kubeconfig is saved in a Kubernetes secret named `target-cluster-kubeconfig` in the same namespace as the Kubeconfig object. Use a [Target or ClusterTarget](scheduling.md#targets-and-cluster-targets) to tell the Admiralty agent to use this kubeconfig for multi-cluster scheduling.

The Admiralty agent uses the Admiralty reference of the other cluster, i.e., its name and account name, to get the other cluster's kube-mtls-proxy address (the reverse tunnel entrance on the Admiralty Cloud API) and its registered CA certificate, which signs the mTLS server certificate used by kube-mtls-proxy at the tunnel exit:

```yaml
spec:
  cluster:
    server: "https://kube-mtls-proxy.target-cluster.user-abcdef.tunnels.api.admiralty.io"
    certificateAuthorityData: "..." # a base64-encoded PEM X.509 certificate
```

If you don't use the Admiralty Cloud API, you can actually omit the `admiraltyReference` and specify `server` and `certificateAuthorityData` directly. Their values will depend on where the other cluster exposes its kube-mtls-proxy.

By default, Kubeconfigs use the certificate secret of the default Identity (see below) of the namespace where they're created as mTLS client certificate and private key. If you want to use a specific certificate secret instead, e.g., from a specific Identity, add this:

```yaml
spec:
  user:
    certificateSecretName: some-cert-secret
```

You can technically use any certificate, as long as it is signed by a CA that is trusted by the other cluster's kube-mtls-proxy (i.e., by a TrustedIdentityProvider object in the other cluster, see below). The Admiralty agent includes a cert-manager Issuer that, among other things, signs Identity certificates. The Admiralty Cloud API facilitates the distribution of that CA's registered certificate to other clusters using Admiralty references, so we recommend you to use Identities, including the default ones.

## Identities

Thanks to the default Identities discussed above, you don't have to create Identities, so feel free to skip this section if you're in a hurry.

Similar to how Kubernetes creates a default ServiceAccount in each namespace, The Admiralty agent creates a default Identity in each namespace _that requires one_ (i.e., if a Kubeconfig in that namespace doesn't specify a client certificate secret name). The default Identity looks like this:

```yaml
apiVersion: multicluster.admiralty.io/v1alpha1
kind: Identity
metadata:
  name: default
  namespace: namespace-a
spec:
  secretName: default-identity-certificate
```

If you need multiple identities in a namespace, e.g., to match right-fit RBAC, you can easily create more. Here's an example:

```yaml
apiVersion: multicluster.admiralty.io/v1alpha1
kind: Identity
metadata:
  name: my-app
  namespace: namespace-a
spec:
  secretName: my-app-id-cert
```

The Admiralty agent delegates the certificate secret creation process to cert-manager, using an Issuer in the installation namespace. Note that we don't use a cert-manager ClusterIssuer so Identity RBAC cannot be bypassed. Only Kubernetes users and service accounts with proper Identity RBAC can generate certificate secrets signed by the Admiralty agent's CA.

The certificate's common name looks like `ns/namespace-a/id/default` or `ns/namespace-a/id/my-app`, respectively. It will be combined with prefixes defined in TrustedIdentityProviders (see below) to build user names to match to RBAC rules in other clusters.

## Trusted Identity Providers

The Admiralty agent's kube-mtls-proxy watches TrustedIdentityProvider objects to assemble a dynamic bundle of trusted CA certificates. Kubernetes API requests transiting through the proxy must present valid client certificates signed by a bundled CA. Here is a sample TrustedIdentityProvider:

```yaml
apiVersion: multicluster.admiralty.io/v1alpha1
kind: TrustedIdentityProvider
metadata:
  name: source-cluster
  namespace: namespace-a
spec:
  prefix: "spiffe://source-cluster/"
  admiraltyReference:
    accountName: user-abcdef # defaults to the cluster's account name
    clusterName: source-cluster
```

The Admiralty agent uses the Admiralty reference of another cluster, i.e., its name and account name, to get the other cluster's registered CA certificate, which signs Identity certificates used as mTLS client certificates, and updates the TrustedIdentityProvider with:

```yaml
spec:
  certificateAuthorityData: "..." # a base64-encoded PEM X.509 certificate
```

If you don't use the Admiralty Cloud API, you can actually omit the `admiraltyReference` and specify `certificateAuthorityData` directly. It can actually be any CA, especially if the other cluster doesn't use Admiralty agent Identities (see above), i.e., if it runs its own public key infrastructure (PKI).

The `prefix` field is important. It is combined with client certificate common names (over which this cluster has no control) to form Kubernetes user names, which are matched to RBAC rules. To avoid unwanted impersonation, it is recommended to use prefixes to isolate TrustedIdentityProviders by trust domain, typically one prefix by TrustedIdentityProvider. By convention (following the [SPIFFE](https://spiffe.io/) standard used by other mTLS projects like service meshes), trust domain prefixes usually start with `spiffe://` and end with a forward slash. Combined with Identity certificate common names (see above), Kubernetes user names look like `spiffe://source-cluster/ns/namespace-a/id/default`. Use them as subjects of RBAC rules to authorize requests. For multi-cluster scheduling, Admiralty generates the right RBAC rules from [Source and ClusterSource](scheduling.md#sources-and-cluster-sources) objects.