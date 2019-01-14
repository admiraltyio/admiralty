set -euo pipefail

RELEASE="$1"

echo "codegen"
hack/codegen.sh
echo "test"
test/test.sh
echo "build"
build/build.sh
echo "e2e test"
test/e2e/e2e.sh
echo "release"
release/release.sh $RELEASE
