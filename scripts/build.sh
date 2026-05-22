#!/usr/bin/env bash

set -euo pipefail

SERVICE_NAME=${SERVICE_NAME:-stasrv}

# ------------------------------------------------
# build configuration
# ------------------------------------------------

BUILD_DIR="cmd/${SERVICE_NAME}"
FILE_NAME="${SERVICE_NAME}$(go env GOEXE)"
GO_VERSION=$(go env GOVERSION)
GOOS=${GOOS:-$(go env GOOS)}
GOARCH=${GOARCH:-$(go env GOARCH)}

# ------------------------------------------------
# git information
# ------------------------------------------------

VERSION="dev"
COMMIT="none"
BUILD_TIME=$(date -u '+%Y-%m-%dT%H:%M:%SZ')

if command -v git >/dev/null 2>&1 && git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
    COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "none")
fi

echo ""
echo "Build Information:"
echo "  Version     : $VERSION"
echo "  Commit      : $COMMIT"
echo "  Build Time  : $BUILD_TIME"
echo "  Go Version  : $GO_VERSION"
echo "  Platform    : ${GOOS}/${GOARCH}"
echo "  Output      : ${BUILD_DIR}/${FILE_NAME}"
echo ""

GOOS=$GOOS GOARCH=$GOARCH CGO_ENABLED=0 go build \
  -o "${BUILD_DIR}/$FILE_NAME" \
  -ldflags " \
  -w \
  -X main.version=$VERSION \
  -X main.buildTime=$BUILD_TIME \
  -X main.commit=$COMMIT" \
  "$BUILD_DIR/main.go"

echo "Build completed successfully."
echo ""
