
# Red Hat OpenShift on IBM Cloud

The [quick start guide](https://admiralty.io/docs/quick_start) provides clear instructions how to use Admiralty on Kubernetes clusters. The only
thing you need to pay special attention to is how to create a kubeconfig secret that would work in your OpenShift cluster on IBM Cloud. This tutorial will
guide you how to create the kubeconfig secret when you use the Red Hat OpenShift on IBM Cloud service.

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
oc get cm trusted-ca-bundle -n openshift-authentication-operator -o yaml | grep -A 22 "DigiCert Global Root CA" | sed 's/^ *//g' | base64 -w0
```
You may need to adjust the number, 22, in the above command. Make sure the last line you capture is the end of this certificate. You then use the output
above for the certificate-authority-data field.

You are now ready to create a kubeconfig secret for the target cluster using the modified kubeconfig file.
```bash
CONFIG=$(oc config view --minify  --raw --output json --kubeconfig ~/.kube/config)
oc create secret generic <secret name> --from-literal=config="$CONFIG"
```
## Summary
In this tutorial, you've learned how to create a kubeconfig secret for the Red Hat OpenShift cluster on IBM Cloud. You can follow the rest of the steps in the [quick start guide](https://admiralty.io/docs/quick_start). Admiralty is integrated and fuctioned in OpenShift clusters. Start exploring Admiralty on IBM Cloud today!
