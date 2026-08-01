#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR"

docker compose up --detach postgres redis

export ROLLBOARD_ADDR="${ROLLBOARD_ADDR:-127.0.0.1:8080}"
export ROLLBOARD_DATABASE_URL="${ROLLBOARD_DATABASE_URL:-postgres://rollboard:rollboard@127.0.0.1:5432/rollboard?sslmode=disable}"
export ROLLBOARD_REDIS_URL="${ROLLBOARD_REDIS_URL:-redis://127.0.0.1:6379/0}"
export ROLLBOARD_APP_ORIGIN="${ROLLBOARD_APP_ORIGIN:-http://127.0.0.1:5173}"

echo "Starting backend on $ROLLBOARD_ADDR (PostgreSQL: $ROLLBOARD_DATABASE_URL)"
cd "$ROOT_DIR/backend"
exec go run ./cmd/server
