#!/usr/bin/env bash
# ui-shots.sh — start the stack and photograph every screen with a real browser.
#
# Writes to artifacts/ui/<theme>/<viewport>/. These are the screenshots used in
# the README, so regenerating them is how the documentation stays honest.
#
#   ./scripts/ui-shots.sh          # English
#   ROLLBOARD_UI_LOCALE=ru ./scripts/ui-shots.sh

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
ADDR="${ROLLBOARD_UI_ADDR:-127.0.0.1:18099}"
DATABASE_URL="postgres://rollboard:rollboard@127.0.0.1:5432/rollboard_test?sslmode=disable"
SERVER_PID=""
SERVER_BIN=""

cleanup() {
  if [ -n "$SERVER_PID" ]; then
    kill "$SERVER_PID" 2>/dev/null || true
    wait "$SERVER_PID" 2>/dev/null || true
  fi
  rm -f "$SERVER_BIN"
  docker compose --project-directory "$ROOT_DIR" down --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

cd "$ROOT_DIR"
"$ROOT_DIR/scripts/test-services.sh"

if [ ! -d frontend/node_modules ]; then (cd frontend && npm ci); fi
(cd frontend && npm run build >/dev/null)

SERVER_BIN="$(mktemp /tmp/rollboard-ui-shots-XXXXXX)"
(cd backend && go build -o "$SERVER_BIN" ./cmd/server)

ROLLBOARD_ADDR="$ADDR" \
ROLLBOARD_DATABASE_URL="$DATABASE_URL" \
ROLLBOARD_REDIS_URL="redis://127.0.0.1:6379/3" \
ROLLBOARD_APP_ORIGIN="http://$ADDR" \
ROLLBOARD_STATIC_DIR="$ROOT_DIR/frontend/dist" \
ROLLBOARD_LOCALES_DIR="$ROOT_DIR/locales" \
"$SERVER_BIN" > /tmp/rollboard-ui-shots.log 2>&1 &
SERVER_PID=$!

for attempt in $(seq 1 30); do
  if curl --noproxy '*' --fail --silent "http://$ADDR/api/health" >/dev/null 2>&1; then break; fi
  if [ "$attempt" -eq 30 ]; then
    echo "backend did not start" >&2
    cat /tmp/rollboard-ui-shots.log >&2
    exit 1
  fi
  sleep 1
done

# Run from frontend/, where Playwright is installed.
cd "$ROOT_DIR/frontend"
ROLLBOARD_UI_URL="http://$ADDR" \
ROLLBOARD_UI_OUT="${ROLLBOARD_UI_OUT:-$ROOT_DIR/artifacts/ui}" \
ROLLBOARD_UI_LOCALE="${ROLLBOARD_UI_LOCALE:-en}" \
  node "$ROOT_DIR/frontend/scripts/ui-shots.mjs"
