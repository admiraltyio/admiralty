set -euo pipefail

if [ $(kubectl --context cluster2 get pod | wc -l) -gt 1 ]
then
	echo "SUCCESS"
	exit 0
else
	echo "FAILURE"
	exit 1
fi
