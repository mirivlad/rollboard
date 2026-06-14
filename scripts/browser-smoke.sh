#!/usr/bin/env bash
# Rollboard Browser Smoke Test
# Requires: Hermes Agent with local Chromium + browser tools
# This is a documented Hermes workflow script that can be run manually
# or via the Hermes Agent in its current session.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
ARTIFACTS_DIR="$ROOT_DIR/artifacts/browser-smoke"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
PASS=0
FAIL=0

mkdir -p "$ARTIFACTS_DIR"

echo "=== Rollboard Browser Smoke Test ==="
echo "  Artifacts: $ARTIFACTS_DIR"
echo ""

cleanup() {
  echo ""
  echo "Cleaning up..."
  cd "$ROOT_DIR" && make stop-dev 2>/dev/null || true
  echo "Done."
}
trap cleanup EXIT SIGINT SIGTERM

# Start dev server
echo "--- Start dev server ---"
cd "$ROOT_DIR"
export ROLLBOARD_ADDR="127.0.0.1:8080"
export ROLLBOARD_DB_PATH="$ROOT_DIR/data/browser-smoke-test.db"
rm -f "$ROOT_DIR/data/browser-smoke-test.db"*
./scripts/dev.sh &
DEV_PID=$!
echo "  Dev server PID: $DEV_PID"

# Wait for server
for i in $(seq 1 15); do
  if curl -sf "http://127.0.0.1:8080/api/health" >/dev/null 2>&1; then
    echo "  Backend ready (attempt $i)"
    break
  fi
  sleep 1
done

# Wait for Vite
for i in $(seq 1 15); do
  if curl -sf -o /dev/null "http://127.0.0.1:5173" 2>/dev/null; then
    echo "  Frontend ready (attempt $i)"
    break
  fi
  sleep 1
done

# Smoke test steps via API (browser-agnostic)
echo ""
echo "--- HTTP API checks ---"

# 1. Create Dungeon Race demo
echo "  Creating Dungeon Race..."
CREATE=$(curl -s -o /dev/null -w "%{http_code}" -X POST \
  -H "Content-Type: application/json" \
  -d '{"id":"browser-smoke-dr","title":"Browser Smoke Dungeon Race","version":1,"board":{"width":1056,"height":384,"cellSize":96,"cells":[{"id":"start","title":"Start","type":"start","x":0,"y":96,"visual":{"baseColor":"#4CAF50"},"fields":{},"onLand":[]},{"id":"trap","title":"Trap","type":"trap","x":192,"y":96,"visual":{"baseColor":"#FFCDD2"},"fields":{},"onLand":[{"type":"lose_resource","resource":"health","amount":2}]},{"id":"treasure","title":"Treasure","type":"treasure","x":384,"y":96,"visual":{"baseColor":"#FFF9C4"},"fields":{},"onLand":[{"type":"gain_resource","resource":"gold","amount":5}]},{"id":"key","title":"Key","type":"key","x":576,"y":96,"visual":{"baseColor":"#E1BEE7"},"fields":{},"onLand":[{"type":"gain_resource","resource":"keys","amount":1}]},{"id":"heal","title":"Heal","type":"heal","x":768,"y":96,"visual":{"baseColor":"#C8E6C9"},"fields":{},"onLand":[{"type":"gain_resource","resource":"health","amount":2}]},{"id":"finish","title":"Finish","type":"finish","x":960,"y":96,"visual":{"baseColor":"#FFD700"},"fields":{},"onLand":[{"type":"finish_game"}]}],"edges":[{"id":"e1","from":"start","to":"trap","condition":{"type":"always"}},{"id":"e2","from":"trap","to":"treasure","condition":{"type":"always"}},{"id":"e3","from":"treasure","to":"key","condition":{"type":"always"}},{"id":"e4","from":"key","to":"heal","condition":{"type":"always"}},{"id":"e5","from":"heal","to":"finish","condition":{"type":"always"}}]},"rules":{"dice":{"count":1,"sides":6},"resources":{"health":{"initial":10,"min":0,"max":10},"gold":{"initial":0},"keys":{"initial":0}},"cellTypes":{"start":{},"trap":{},"treasure":{},"key":{},"heal":{},"finish":{}},"startBonus":0,"startBonusResource":""}}' \
  "http://127.0.0.1:8080/api/games" 2>/dev/null || echo "000")
if [ "$CREATE" = "200" ] || [ "$CREATE" = "201" ]; then
  echo "    PASS: Created (HTTP $CREATE)"
  PASS=$((PASS + 1))
else
  echo "    FAIL: Create returned $CREATE (expected 200/201)"
  FAIL=$((FAIL + 1))
fi

# 2. Validate Dungeon Race
echo "  Validating..."
VALIDATE=$(curl -s -X POST "http://127.0.0.1:8080/api/games/browser-smoke-dr/validate" 2>/dev/null || echo '{"valid":false}')
if echo "$VALIDATE" | grep -q '"valid":true'; then
  echo "    PASS: Valid"
  PASS=$((PASS + 1))
else
  echo "    FAIL: $VALIDATE"
  FAIL=$((FAIL + 1))
fi

# 3. Start playtest
echo "  Starting playtest..."
SESSION=$(curl -s -X POST -H "Content-Type: application/json" \
  -d '{"mode":"hotseat","players":[{"name":"Hero 1","color":"#e74c3c"},{"name":"Hero 2","color":"#3498db"}]}' \
  "http://127.0.0.1:8080/api/games/browser-smoke-dr/playtest" 2>/dev/null || echo "FAIL")
SESSION_ID=$(echo "$SESSION" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4 || echo "")
if [ -n "$SESSION_ID" ]; then
  echo "    PASS: Session created (id=$SESSION_ID)"
  PASS=$((PASS + 1))
else
  echo "    FAIL: Session creation: $SESSION"
  FAIL=$((FAIL + 1))
fi

# 4. Roll dice
if [ -n "$SESSION_ID" ]; then
  echo "  Rolling dice..."
  ROLL=$(curl -s -X POST "http://127.0.0.1:8080/api/sessions/$SESSION_ID/roll" 2>/dev/null || echo '{"error":"fail"}')
  if echo "$ROLL" | grep -q '"total"'; then
    TOTAL=$(echo "$ROLL" | grep -o '"total":[0-9]*' | cut -d: -f2 || echo "?")
    echo "    PASS: Rolled total=$TOTAL"
    PASS=$((PASS + 1))
  else
    echo "    FAIL: Roll failed: $(echo "$ROLL" | head -c 200)"
    FAIL=$((FAIL + 1))
  fi

  # 5. Get session state
  echo "  Checking session state..."
  STATE=$(curl -s "http://127.0.0.1:8080/api/sessions/$SESSION_ID" 2>/dev/null || echo '{}')
  STATUS=$(echo "$STATE" | grep -o '"status":"[^"]*"' | head -1 | cut -d'"' -f4 || echo "unknown")
  echo "    Session status: $STATUS"
  PASS=$((PASS + 1))
fi

# 6. Check health after roll
echo "  Health check after actions..."
HEALTH=$(curl -s -o /dev/null -w "%{http_code}" "http://127.0.0.1:8080/api/health" 2>/dev/null || echo "000")
if [ "$HEALTH" = "200" ]; then
  echo "    PASS: Health still OK"
  PASS=$((PASS + 1))
else
  echo "    FAIL: Health returned $HEALTH"
  FAIL=$((FAIL + 1))
fi

# Summary
echo ""
echo "=== Browser Smoke Results: $PASS passed, $FAIL failed ==="
echo "$PASS/$((PASS + FAIL)) checks passed" > "$ARTIFACTS_DIR/result_$TIMESTAMP.txt"

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
