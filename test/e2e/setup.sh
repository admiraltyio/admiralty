set -euo pipefail

# Install MCSA and bootstrap cluster1 and cluster2 to import from cluster1 (which will host the control plane)
RELEASE_URL=https://github.com/admiraltyio/multicluster-service-account/releases/download/v0.2.1
MCSA_URL="$RELEASE_URL/install.yaml"
kubectl --context cluster1 apply -f "$MCSA_URL"
kubectl --context cluster2 apply -f "$MCSA_URL"
# TODO: don't assume the right kubemcsa is installed
sleep 15 # TODO: fix race condition: kubemcsa assumes pod admission controller is ready (automount webhook)
kubemcsa bootstrap cluster1 cluster1
kubemcsa bootstrap cluster2 cluster1

# Install Argo in cluster1
kubectl --context cluster1 create ns argo
kubectl --context cluster1 apply -n argo -f https://raw.githubusercontent.com/argoproj/argo/v2.2.1/manifests/install.yaml
kubectl --context cluster1 apply -f config/samples/argo-workflows/_service-account.yaml
# the workflow service account must exist in the other cluster
kubectl --context cluster2 apply -f config/samples/argo-workflows/_service-account.yaml

kubectl config use-context cluster1 && skaffold run -f test/e2e/scheduler/skaffold.yaml
kustomize build test/e2e/federation | kubectl --context cluster1 apply -f -
kubectl config use-context cluster1 && skaffold run -f test/e2e/agent1/skaffold.yaml

kubectl config use-context cluster2 && skaffold run -f test/e2e/agent2/skaffold.yaml
# TODO: skaffold deploy rather than run, because images have already been built for cluster1 (need to pass tagged image names)

# switch back to cluster1 for user commands afterward
# TODO save current context at beginning and return to it
kubectl config use-context cluster1

argo --context cluster1 submit --serviceaccount argo-workflow --watch config/samples/argo-workflows/blog-scenario-a-multicluster.yaml
