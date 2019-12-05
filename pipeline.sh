#!/usr/bin/env bash
set -euo pipefail

VERSION="$1"

echo "test"
test/test.sh
echo "build"
build/build.sh "$VERSION"
echo "e2e test"
test/e2e/e2e.sh "$VERSION"
echo "release"
release/release.sh "$VERSION"
