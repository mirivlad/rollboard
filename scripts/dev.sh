#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
BACKEND_PID=""
FRONTEND_PID=""

cleanup() {
  for pid in "$BACKEND_PID" "$FRONTEND_PID"; do
    if [ -n "$pid" ]; then kill "$pid" 2>/dev/null || true; fi
  done
  for pid in "$BACKEND_PID" "$FRONTEND_PID"; do
    if [ -n "$pid" ]; then wait "$pid" 2>/dev/null || true; fi
  done
  rm -f /tmp/rollboard-dev-backend.pid /tmp/rollboard-dev-frontend.pid
}
trap cleanup EXIT INT TERM

cd "$ROOT_DIR"
docker compose up --detach postgres redis
if [ ! -d frontend/node_modules ]; then (cd frontend && npm install); fi
setsid bash -c "cd '$ROOT_DIR/backend' && \
ROLLBOARD_ADDR='${ROLLBOARD_ADDR:-127.0.0.1:8080}' \
ROLLBOARD_DATABASE_URL='${ROLLBOARD_DATABASE_URL:-postgres://rollboard:rollboard@127.0.0.1:5432/rollboard?sslmode=disable}' \
ROLLBOARD_REDIS_URL='${ROLLBOARD_REDIS_URL:-redis://127.0.0.1:6379/0}' \
ROLLBOARD_APP_ORIGIN='${ROLLBOARD_APP_ORIGIN:-http://127.0.0.1:5173}' \
exec go run ./cmd/server" &
BACKEND_PID=$!
printf '%s\n' "$BACKEND_PID" > /tmp/rollboard-dev-backend.pid
setsid bash -c "cd '$ROOT_DIR/frontend' && exec ./node_modules/.bin/vite --host 127.0.0.1" &
FRONTEND_PID=$!
printf '%s\n' "$FRONTEND_PID" > /tmp/rollboard-dev-frontend.pid
wait -n "$BACKEND_PID" "$FRONTEND_PID"
