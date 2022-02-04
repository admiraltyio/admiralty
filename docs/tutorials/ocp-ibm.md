
# Red Hat OpenShift on IBM Cloud

The [quick start guide](https://admiralty.io/docs/quick_start) provides clear instructions how to use Admiralty on Kubernetes clusters. The only
thing you need to pay special attention to is how to create a kubeconfig secret that would work in your OpenShift cluster on IBM Cloud. This tutorial will
guide you how to create the kubeconfig secret when you use the Red Hat OpenShift on IBM Cloud service as one of your target clusters. The source cluster can be a Kubernetes or OpenShift cluster.

## Prerequisites
- the [Red Hat OpenShift on IBM Cloud service](https://www.ibm.com/cloud/openshift) 
- the required [CLI tools](https://cloud.ibm.com/docs/openshift?topic=openshift-openshift-cli) ( e.g., IBMCLOUD CLI and OpenShift CLI (oc) )

## Kubeconfig for Authentication
You can follow this [link](https://cloud.ibm.com/docs/openshift?topic=openshift-access_cluster) to access your OpenShift cluster.
After you connect to the OpenShift cluster, you can use the following IBMCLOUD CLI command
to retrieve the kubeconfig file.
```bash
export KUBECONFIG=~/.kube/config

ibmcloud oc cluster config --cluster <your cluster name> --admin
```
Your config file should look like the following:
``` 
apiVersion: v1
clusters:
- cluster:
    server: <api server>
  name: <cluster name>
contexts:
- context:
    cluster: <cluster name>
    namespace: default
    user: <admin user name>
  name: <context name>
current-context: <context name>
kind: Config
preferences: {}
users:
- name: <admin user name>
  user:
    client-certificate: <path to client certificate>
    client-key: <path to client key>
```
Let's modify the config file to the following format below:
```
apiVersion: v1
clusters:
- cluster:
    server: <api server>
    certificate-authority-data: <ca data>
  name: <cluster name>
contexts:
- context:
    cluster: <cluster name>
    namespace: default
    user: <admin user name>
  name: <context name>
current-context: <context name>
kind: Config
preferences: {}
users:
- name: <admin user name>
  user:
    token: <service account token>
```
The fields, client-certificate and client-key, are being removed and certificate-authority-data and token fields are added.

For the token part, you can follow the instructions in the [quick start guide](https://admiralty.io/docs/quick_start) to get the service account token. 

To get the certificate-authority-data, you can use the command below to get the encoded CA data.
```bash
CA_DATA="$(curl https://cacerts.digicert.com/DigiCertGlobalRootCA.crt.pem | base64 -w0)"
```

You are now ready to create a kubeconfig secret for the target cluster. Use the commands below to automate the entire process and have the new kubeconfig content stored in the CONFIG variable:

On the OpenShift target cluster:

```bash
# the namespace where a service account is created
NS=myproj
# the name of your service account
SA_NAME=sa-$NS

# the secret name for your service account
SECRET_NAME=$(oc get serviceaccount $SA_NAME -n $NS --output json | jq -r '.secrets[] | select(.name | contains("token"))' | jq -r '.name')
# the token in the secret
TOKEN=$(oc get secret $SECRET_NAME -n $NS --output json | jq -r '.data.token' | base64 --decode)

# the CA data
CA_CERT="$(curl https://cacerts.digicert.com/DigiCertGlobalRootCA.crt.pem | base64 -w0)"

CONFIG=$( oc config view --minify --raw --output json | jq '.clusters[0].cluster["certificate-authority-data"] = "'$CA_DATA'" | del(.clusters[0].cluster."certificate-authority")' | jq '.users[0].user={token:"'$TOKEN'"}' )
```

On the source cluster, you can then create a secret for the target cluster, where its kubeconfig is stored in the $CONFIG variable.
```
oc create secret generic <secret_name_for_target_cluster> --from-literal=config="$CONFIG"
```
You may also need to adjust the security context constraints (SCCs) as your pod may be configured with the restricted SCC by default in OpenShift. Run the command below when using OpenShift on IBM Cloud:
```
oc adm policy add-scc-to-user ibm-anyuid-scc -z default -n <namespace of your service account>
```

## Summary
In this tutorial, you've learned how to create a kubeconfig secret for the Red Hat OpenShift cluster on IBM Cloud. You can follow the rest of the steps in the [quick start guide](https://admiralty.io/docs/quick_start) to use Admiralty on OpenShift.
