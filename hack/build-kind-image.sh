#!/bin/bash -e

REGISTRY=$1
ARCH=$2
OS=$3
DOCKER_VERSION=$4
KIND_VERSION=$5
shift 5
if [ -n "${REGISTRY}" ]; then
	REGISTRY+="/"
fi
docker build \
	-f kind.Dockerfile \
	--build-arg "OS=${OS}" \
	--build-arg "ARCH=${ARCH}" \
	--build-arg "DOCKER_VERSION=${DOCKER_VERSION}" \
	--build-arg "KIND_VERSION=${KIND_VERSION}" \
	--tag "${REGISTRY}meln5674/kind:${KIND_VERSION}-docker-${DOCKER_VERSION}" \
	"$@" \
	.
