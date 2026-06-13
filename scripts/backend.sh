#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/../backend"

ROLLBOARD_ADDR="${ROLLBOARD_ADDR:-127.0.0.1:8080}"
ROLLBOARD_DB_PATH="${ROLLBOARD_DB_PATH:-./data/rollboard.db}"

mkdir -p "$(dirname "$ROLLBOARD_DB_PATH")"

echo "Starting backend on $ROLLBOARD_ADDR (DB: $ROLLBOARD_DB_PATH)"
go run ./cmd/server/ -addr "$ROLLBOARD_ADDR" -dsn "$ROLLBOARD_DB_PATH"
