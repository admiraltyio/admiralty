set -euo pipefail

source test/e2e/aliases.sh

setup_argo() {
  # Install Argo in cluster1
  k1 create ns argo
  k1 apply -n argo -f https://raw.githubusercontent.com/argoproj/argo/v2.2.1/manifests/install.yaml

  # kind uses containerd not docker so we change the argo executor (default: docker)
  # TODO modify install.yaml instead
  k1 patch cm -n argo workflow-controller-configmap --patch '{"data":{"config":"{\"containerRuntimeExecutor\":\"kubelet\"}"}}'
  k1 delete pod --all -n argo # reload config map

  k1 apply -f examples/argo-workflows/_service-account.yaml
  # the workflow service account must exist in the other cluster
  k2 apply -f examples/argo-workflows/_service-account.yaml

  k1 label ns default multicluster-scheduler=enabled

  # TODO download only if not present or version mismatch
  curl -Lo argo https://github.com/argoproj/argo/releases/download/v2.2.1/argo-linux-amd64
  chmod +x argo

  # speed up container creations
  docker pull argoproj/argoexec:v2.2.1 # may already be on host
  kind load docker-image argoproj/argoexec:v2.2.1 --name cluster1
  kind load docker-image argoproj/argoexec:v2.2.1 --name cluster2
}

tear_down_argo() {
  k1 label ns default multicluster-scheduler-

  k2 delete -f examples/argo-workflows/_service-account.yaml
  k1 delete -f examples/argo-workflows/_service-account.yaml
  k1 delete -n argo -f https://raw.githubusercontent.com/argoproj/argo/v2.2.1/manifests/install.yaml
  k1 delete ns argo
}

test_blog_scenario_a_multicluster() {
  KUBECONFIG=kubeconfig-cluster1 ./argo submit --serviceaccount argo-workflow --watch examples/argo-workflows/blog-scenario-a-multicluster.yaml

  if [ $(k2 get pod | wc -l) -gt 1 ]; then
    echo "SUCCESS"
  else
    echo "FAILURE"
    exit 1
  fi

  KUBECONFIG=kubeconfig-cluster1 ./argo delete --all
}
