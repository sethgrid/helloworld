#!/usr/bin/env bash
# Generate HTTP traffic against a local helloworld (default http://127.0.0.1:16666).
set -euo pipefail
BASE="${HELLOWORLD_URL:-http://127.0.0.1:16666}"
echo "Sending requests to ${BASE} (Ctrl+C to stop)"
while true; do
  curl -sS -o /dev/null "${BASE}/" || true
  curl -sS -o /dev/null "${BASE}/?delay=10ms" || true
  sleep 0.2
done
