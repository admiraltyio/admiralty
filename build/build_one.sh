#!/usr/bin/env bash
#
# Copyright 2021 The Multicluster-Scheduler Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

set -euo pipefail

# constants
default_bin_prefix=multicluster-scheduler-

# environment variables
# required
PKG="${PKG}"
# optional
BIN="${BIN:-$default_bin_prefix$(basename "$PKG")}"
OS="${OS:-linux}"
ARCH="${ARCH:-amd64}"
BUILD_IMG="${BUILD_IMG:-true}"
IMG="${IMG:-$BIN}"
BASE_IMG="${BASE_IMG:-scratch}"
VERSION="${VERSION:-dev}"
LDFLAGS="${LDFLAGS:-}"
DEBUG="${DEBUG:-false}"
OUTPUT_TAR_GZ="${OUTPUT_TAR_GZ:-false}"
CR="${CR:-docker.io/library}"

extra_args=()
if [ -n "$LDFLAGS" ]; then
  extra_args+=("-ldflags" "$LDFLAGS")
fi
if [ "$DEBUG" = true ]; then
  extra_args+=("-gcflags=all=-N -l")
fi

context_dir="_out/$PKG/run/$OS/$ARCH"
if [ "$DEBUG" = true ]; then
  context_dir="_out/$PKG/debug/$OS/$ARCH"
fi

mkdir -p "$context_dir"

CGO_ENABLED=0 GOOS=$OS GOARCH=$ARCH go build -trimpath -o "$context_dir/$BIN" "${extra_args[@]}" "$PKG"
if [ "$VERSION" != dev ] && [ "$ARCH" = amd64 ]; then
  upx-ucl "$context_dir/$BIN"
fi

if [ "$BUILD_IMG" = true ]; then
  if [ "$DEBUG" = true ]; then
    cp "$(command -v dlv)" "$context_dir" # `command -v` is the portable version of `which`
    cat <<EOF >"$context_dir/Dockerfile"
FROM $BASE_IMG
COPY dlv /usr/local/bin/dlv
COPY $BIN /usr/local/bin/$BIN
ENTRYPOINT ["dlv", "--listen=:2345", "--headless=true", "--api-version=2", "--accept-multiclient", "exec", "--continue", "/usr/local/bin/$BIN", "--"]
EOF
  else
    cat <<EOF >"$context_dir/Dockerfile"
FROM $BASE_IMG
COPY $BIN /usr/local/bin/$BIN
ENTRYPOINT ["$BIN"]
EOF
  fi

  cat "$context_dir"/Dockerfile

  docker build -t "$CR/$IMG:$VERSION-SARCH" "$context_dir"
  docker push "$CR/$IMG:$VERSION-$ARCH"

  if [ "$OUTPUT_TAR_GZ" = true ]; then
    mkdir -p images
    docker save "$IMG:$VERSION-$ARCH" | gzip > images/"$IMG"_"$VERSION-$ARCH".tar.gz
  fi
fi
