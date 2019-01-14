set -euo pipefail

# Create cluster1 and cluster2
PROJECT=$(gcloud config get-value project)
REGION=$(gcloud config get-value compute/zone)
for NAME in cluster1 cluster2; do
	gcloud container clusters create $NAME --preemptible
	gcloud container clusters get-credentials $NAME
	CONTEXT=gke_$PROJECT"_"$REGION"_"$NAME
	sed -i -e "s/$CONTEXT/$NAME/g" ~/.kube/config
	kubectl create clusterrolebinding cluster-admin-binding --clusterrole cluster-admin --user $(gcloud config get-value account)
	kubectl cluster-info
done
