#!/usr/bin/env bash

set -euo pipefail

# Decide on a cleanup strategy based on environmental variables
if [ "${ALL}" = "true" ]; then
    echo "Performing full system prune (all unused resources)..."
    docker system prune -af
else
    echo "Cleaning up stopped containers and dangling images..."
    docker container prune -f
    docker image prune -f

    if [ "${NOCACHE}" = "true" ]; then
        echo "Cleaning build cache..."
        docker builder prune -f
    fi
fi

echo "Docker cleanup completed."
