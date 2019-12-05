k1() { KUBECONFIG=kubeconfig-cluster1 kubectl "$@"; }
k2() { KUBECONFIG=kubeconfig-cluster2 kubectl "$@"; }
k3() { KUBECONFIG=kubeconfig-cluster3 kubectl "$@"; }
helm1() { KUBECONFIG=kubeconfig-cluster1 helm "$@"; }
helm2() { KUBECONFIG=kubeconfig-cluster2 helm "$@"; }
helm3() { KUBECONFIG=kubeconfig-cluster3 helm "$@"; }
