set -euo pipefail

for NAME in cluster1 cluster2; do
	gcloud container clusters delete $NAME --quiet
	kubectl config delete-cluster $NAME
	kubectl config delete-context $NAME
	kubectl config unset users.$NAME
done
