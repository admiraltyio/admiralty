set -euo pipefail

if (kubectl --context cluster2 get pod | grep hello-world)
then
	echo "SUCCESS"
	exit 0
else
	echo "FAILURE"
	exit 1
fi
