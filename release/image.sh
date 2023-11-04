#!/usr/bin/env bash
#
# Copyright 2023 The Multicluster-Scheduler Authors.
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
default_registry="public.ecr.aws/admiralty"
default_archs="amd64 arm64 ppc64le s390x"

# environment variables
# required
IMG="${IMG}"
VERSION="${VERSION}"
# optional
REGISTRY="${REGISTRY:-$default_registry}"
ARCHS="${ARCHS:-$default_archs}"

aws ecr-public get-login-password --region us-east-1 | docker login --username AWS --password-stdin public.ecr.aws

read -ra archs <<<"$ARCHS"

arch_imgs=()
for arch in "${archs[@]}"; do
  arch_img="$REGISTRY/$IMG:$VERSION-$arch"
  docker tag "$IMG:$VERSION-$arch" "$arch_img"
  docker push "$arch_img"
  arch_imgs+=("$arch_img")
done

export DOCKER_CLI_EXPERIMENTAL=enabled

docker manifest create "$REGISTRY/$IMG:$VERSION" "${arch_imgs[@]}"
for arch in "${archs[@]}"; do
  docker manifest annotate --arch "$arch" "$REGISTRY/$IMG:$VERSION" "$REGISTRY/$IMG:$VERSION-$arch"
done
docker manifest push --purge "$REGISTRY/$IMG:$VERSION"
