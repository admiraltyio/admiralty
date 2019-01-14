set -euo pipefail

kubectl config use-context cluster1 && argo delete --all
kubectl config use-context cluster2 && skaffold delete -f test/e2e/agent2/skaffold.yaml
kubectl config use-context cluster1 && skaffold delete -f test/e2e/agent1/skaffold.yaml
kustomize build test/e2e/federation | kubectl --context cluster1 delete -f -
kubectl config use-context cluster1 && skaffold delete -f test/e2e/scheduler/skaffold.yaml
kubectl --context cluster2 delete -f config/samples/argo-workflows/_service-account.yaml
kubectl --context cluster1 delete -f config/samples/argo-workflows/_service-account.yaml
kubectl --context cluster1 delete -n argo -f https://raw.githubusercontent.com/argoproj/argo/v2.2.1/manifests/install.yaml
kubectl --context cluster1 delete ns argo
RELEASE_URL=https://github.com/admiraltyio/multicluster-service-account/releases/download/v0.2.1
MCSA_URL="$RELEASE_URL/install.yaml"
kubectl --context cluster2 delete -f "$MCSA_URL"
kubectl --context cluster1 delete -f "$MCSA_URL"
