#!/usr/bin/env sh
set -eu

export PORT="${PORT:-8080}"
export IKMAN_BASE_URL="${IKMAN_BASE_URL:-https://ikman.lk}"
export IKMAN_REQUEST_INTERVAL="${IKMAN_REQUEST_INTERVAL:-200ms}"
export IKMAN_LOAD_PHONES="${IKMAN_LOAD_PHONES:-true}"
export IKMAN_CALL_DB="${IKMAN_CALL_DB:-data/calls.json}"

printf 'Starting ikman public ads browser at http://localhost:%s\n' "$PORT"
exec go run ./cmd/ikman-browser
