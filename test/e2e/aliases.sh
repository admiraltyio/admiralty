k1() { kubectl --context cluster1 "$@"; }
k2() { kubectl --context cluster2 "$@"; }
c1() { kubectl config use-context cluster1 "$@"; }
c2() { kubectl config use-context cluster2 "$@"; }
