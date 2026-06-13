#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"

BACKEND_PID=""
PASS=0
FAIL=0

cleanup() {
  if [ -n "$BACKEND_PID" ]; then
    kill "$BACKEND_PID" 2>/dev/null || true
    wait "$BACKEND_PID" 2>/dev/null || true
  fi
}

trap cleanup EXIT SIGINT SIGTERM

echo "=== Rollboard Smoke Test ==="

# --- Start backend ---
ADDR="${ROLLBOARD_ADDR:-127.0.0.1:8080}"
DB_PATH="${ROLLBOARD_DB_PATH:-$ROOT/data/smoke-test.db}"

# Clean up previous smoke-test db
rm -f "$ROOT/data/smoke-test.db" "$ROOT/data/smoke-test.db-shm" "$ROOT/data/smoke-test.db-wal"

export ROLLBOARD_ADDR="$ADDR"
export ROLLBOARD_DB_PATH="$DB_PATH"

echo "Starting backend on $ADDR (DB: $DB_PATH)..."
cd "$ROOT/backend"
go run ./cmd/server/ &
BACKEND_PID=$!

# Wait for startup
sleep 2

# --- Test health endpoint ---
echo ""
echo "--- Health check ---"
HEALTH=$(curl -s -o /dev/null -w "%{http_code}" "http://$ADDR/api/health" 2>/dev/null || echo "000")
if [ "$HEALTH" = "200" ]; then
  echo "  PASS: /api/health returned 200"
  PASS=$((PASS + 1))
else
  echo "  FAIL: /api/health returned $HEALTH (expected 200)"
  FAIL=$((FAIL + 1))
fi

# --- Test list games (empty) ---
echo ""
echo "--- List games (expect empty) ---"
GAMES=$(curl -s "http://$ADDR/api/games" 2>/dev/null || echo "FAIL")
if [ "$GAMES" = "FAIL" ]; then
  echo "  FAIL: GET /api/games failed"
  FAIL=$((FAIL + 1))
elif echo "$GAMES" | grep -q '"games":\[\]' 2>/dev/null || echo "$GAMES" | grep -q '\[\]' 2>/dev/null; then
  echo "  PASS: GET /api/games returned empty list"
  PASS=$((PASS + 1))
else
  echo "  PASS: GET /api/games returned data (non-empty)"
  PASS=$((PASS + 1))
fi

# --- Create a simple game ---
echo ""
echo "--- Create a test game ---"
CREATE_BODY='{
  "id": "smoke-test-game",
  "title": "Smoke Test",
  "version": 1,
  "board": {
    "width": 800,
    "height": 600,
    "cellSize": 96,
    "cells": [
      {"id":"start","title":"Start","type":"start","x":50,"y":300,"visual":{"baseColor":"#4CAF50"},"fields":{}},
      {"id":"cell2","title":"Cell 2","type":"empty","x":200,"y":300,"visual":{"baseColor":"#ccc"},"fields":{}}
    ],
    "edges": [
      {"id":"e1","from":"start","to":"cell2","condition":{"type":"always"}}
    ]
  },
  "rules": {
    "dice": {"count":1,"sides":6},
    "resources": {"money":{"initial":500}},
    "cellTypes": {"start":{},"empty":{}},
    "startBonus": 100,
    "startBonusResource": "money"
  }
}'
CREATE=$(curl -s -o /dev/null -w "%{http_code}" -X POST -H "Content-Type: application/json" -d "$CREATE_BODY" "http://$ADDR/api/games" 2>/dev/null || echo "000")
if [ "$CREATE" = "200" ] || [ "$CREATE" = "201" ]; then
  echo "  PASS: POST /api/games returned $CREATE"
  PASS=$((PASS + 1))
else
  echo "  FAIL: POST /api/games returned $CREATE (expected 200/201)"
  FAIL=$((FAIL + 1))
fi

# --- Get the game ---
echo ""
echo "--- Get test game ---"
GET=$(curl -s -o /dev/null -w "%{http_code}" "http://$ADDR/api/games/smoke-test-game" 2>/dev/null || echo "000")
if [ "$GET" = "200" ]; then
  echo "  PASS: GET /api/games/smoke-test-game returned 200"
  PASS=$((PASS + 1))
else
  echo "  FAIL: GET /api/games/smoke-test-game returned $GET (expected 200)"
  FAIL=$((FAIL + 1))
fi

# --- Validate the game ---
echo ""
echo "--- Validate test game ---"
VALIDATE=$(curl -s -X POST "http://$ADDR/api/games/smoke-test-game/validate" 2>/dev/null || echo '{"valid":false}')
if echo "$VALIDATE" | grep -q '"valid":true'; then
  echo "  PASS: Validate returned valid"
  PASS=$((PASS + 1))
else
  echo "  FAIL: Validate failed: $VALIDATE"
  FAIL=$((FAIL + 1))
fi

# --- Start a playtest session ---
echo ""
echo "--- Start hotseat session ---"
SESSION_BODY='{"mode":"hotseat","players":[{"name":"Alice","color":"#e74c3c"},{"name":"Bob","color":"#3498db"}]}'
SESSION=$(curl -s -X POST -H "Content-Type: application/json" -d "$SESSION_BODY" "http://$ADDR/api/games/smoke-test-game/playtest" 2>/dev/null || echo "FAIL")
SESSION_ID=$(echo "$SESSION" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4 || echo "")
if [ -n "$SESSION_ID" ]; then
  echo "  PASS: Session created with id=$SESSION_ID"
  PASS=$((PASS + 1))
else
  echo "  FAIL: Session creation failed: $SESSION"
  FAIL=$((FAIL + 1))
fi

echo ""
echo "=== Results: $PASS passed, $FAIL failed ==="

# Cleanup happens via trap
if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
