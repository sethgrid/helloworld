#!/usr/bin/env bash
set -o pipefail
set -e

echo "clearing db..."
docker compose stop db || true
yes | docker compose rm -f db 2> /dev/null || true

echo "running integration tests..."
docker compose up --no-deps --build integration

echo "success!"
