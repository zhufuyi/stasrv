#!/usr/bin/env bash

set -euo pipefail

SERVER_NAME=${SERVER_NAME:-stasrv}

# ------------------------------------------------------------
# parameter analyzing
# ------------------------------------------------------------

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DOCKERFILE="${SCRIPT_DIR}/Dockerfile"
CONTEXT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)" # Build context directory (same level as go.mod)

# get image name from env first
IMAGE_NAME="${IMAGE_NAME:-}"
if [[ -z "$IMAGE_NAME" ]]; then
    IMAGE_NAME=${SERVER_NAME}
fi

REGISTRY="${1:-}"
TAG="${2:-}"
PUSH="${3:-true}"   # default is push

if [[ -z "$REGISTRY" || -z "$TAG" ]]; then
    echo "ERROR: Both REGISTRY and TAG are required."
    echo "Usage: $0 <registry> <tag> [push]"
    echo "  push : true (default) to build & push, false to build & load locally"
    exit 1
fi

FULL_IMAGE="${REGISTRY}/${IMAGE_NAME}"
PUSH=$(echo "$PUSH" | tr '[:upper:]' '[:lower:]')

# ------------------------------------------------------------
# Unified image tag
# ------------------------------------------------------------

TAG_ARGS=("--tag" "${FULL_IMAGE}:${TAG}")

# When the incoming TAG is not the latest, always add the latest tag.
if [[ "$TAG" != "latest" ]]; then
    TAG_ARGS+=("--tag" "${FULL_IMAGE}:latest")
fi


# ------------------------------------------------------------
# Build and Push
# ------------------------------------------------------------

echo ""
echo "CONTEXT_DIR:  ${CONTEXT_DIR}"
echo "DOCKERFILE:   ${DOCKERFILE}"
echo "IMAGE:        ${FULL_IMAGE}:${TAG}"
echo ""

if ! docker buildx version > /dev/null 2>&1; then
    echo "ERROR: docker buildx is required but not available."
    exit 1
fi

if [[ "$PUSH" == "true" ]]; then
    BUILDER_NAME="${IMAGE_NAME}-multiarch"
    if ! docker buildx inspect "${BUILDER_NAME}" > /dev/null 2>&1; then
        echo "Creating buildx builder: ${BUILDER_NAME}"
        docker buildx create --name "${BUILDER_NAME}" --use --bootstrap
    else
        docker buildx use "${BUILDER_NAME}"
    fi

    PLATFORMS="linux/amd64,linux/arm64"
    echo "Building and pushing multi-arch image, platforms: ${PLATFORMS}"
    docker buildx build \
        --platform "${PLATFORMS}" \
        --push \
        "${TAG_ARGS[@]}" \
        --file "${DOCKERFILE}" \
        "${CONTEXT_DIR}"
    echo "Multi-arch image pushed successfully."
else
    docker buildx use default
    echo "Building and loading image locally (single-platform)"
    docker buildx build \
        "${TAG_ARGS[@]}" \
        --load \
        --file "${DOCKERFILE}" \
        "${CONTEXT_DIR}"
    echo "Image loaded to local Docker daemon."
fi

echo "  Tags:"
for ((i=1; i<${#TAG_ARGS[@]}; i+=2)); do
    echo "    ${TAG_ARGS[i]}"
done
