set -euo pipefail

RELEASE="$1"

DEPLOYMENTS=("admiralty" "multicluster-service-account" "scheduler")
for DEPLOYMENT in "${DEPLOYMENTS[@]}"; do
	sed "s/RELEASE/$RELEASE/g" "release/$DEPLOYMENT/kustomization.tmpl.yaml" > "release/$DEPLOYMENT/kustomization.yaml"
done

kustomize build release/admiralty -o _out/admiralty.yaml
kustomize build release/multicluster-service-account -o _out/agent.yaml
kustomize build release/scheduler -o _out/scheduler.yaml
# TODO: upload to GitHub

RELEASE=$RELEASE skaffold build -f release/skaffold.yaml
# TODO: also tag images with latest
