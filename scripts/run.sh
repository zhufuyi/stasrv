#!/usr/bin/env bash

set -euo pipefail

SERVICE_NAME=${SERVICE_NAME:-stasrv}

# ------------------------------------------
# configuration
# ------------------------------------------
BINARY_FILE="cmd/${SERVICE_NAME}/${SERVICE_NAME}"
BUILD=${BUILD:-true}

DIR=${DIR:-}
BASE_PATH=${BASE_PATH:-/}
PORT=${PORT:-8080}
#CONFIG_FILE=${CONFIG:-}

# ------------------------------------------
# build step
# ------------------------------------------
if [[ "$BUILD" == "true" ]]; then
    bash scripts/build.sh
fi

# ------------------------------------------
# check binary
# ------------------------------------------
if [[ ! -f "$BINARY_FILE" ]]; then
    echo "Error: binary not found: $BINARY_FILE"
    echo "Please run build first."
    exit 1
fi

# ------------------------------------------
# run server
# ------------------------------------------
"./$BINARY_FILE" --dir="$DIR" --base-path="$BASE_PATH" --port="$PORT"
