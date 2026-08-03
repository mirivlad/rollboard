#!/usr/bin/env bash
# validate-demos.sh — verify that demo game definitions pass backend validation.
#
# Usage:  ./scripts/validate-demos.sh
#
# Starts the backend on a random port with the dedicated PostgreSQL test DB and Redis,
# POSTs each demo, calls the validate endpoint, and cleans up.

set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PASS=0
FAIL=0

PORT=$(( (RANDOM % 1000) + 9000 ))
ADDR="127.0.0.1:${PORT}"
DATABASE_URL="postgres://rollboard:rollboard@127.0.0.1:5432/rollboard_test?sslmode=disable"
REDIS_URL="redis://127.0.0.1:6379/0"
BACKEND_PID=""
SERVER_BIN=""
COOKIE_JAR="$(mktemp)"

cleanup() {
  if [ -n "$BACKEND_PID" ]; then
    kill "$BACKEND_PID" 2>/dev/null || true
    wait "$BACKEND_PID" 2>/dev/null || true
  fi
  rm -f "$SERVER_BIN" "$COOKIE_JAR" 2>/dev/null || true
  docker compose --project-directory "$ROOT" down --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

echo "=== Demo Validation Check ==="
echo "Starting backend on $ADDR (PostgreSQL test DB and Redis)..."

docker compose --project-directory "$ROOT" up --detach postgres redis >/dev/null
for i in $(seq 1 30); do
  if docker compose --project-directory "$ROOT" exec -T postgres pg_isready -U rollboard -d rollboard_test >/dev/null 2>&1; then
    break
  fi
  if [ "$i" -eq 30 ]; then
    echo "  FAIL: PostgreSQL test database did not become ready"
    exit 1
  fi
  sleep 1
done

for i in $(seq 1 30); do
  if docker compose --project-directory "$ROOT" exec -T redis redis-cli ping >/dev/null 2>&1; then
    break
  fi
  if [ "$i" -eq 30 ]; then
    echo "  FAIL: Redis did not become ready"
    exit 1
  fi
  sleep 1
done

# Build server binary first to avoid go-run subprocess leaks
SERVER_BIN=$(mktemp /tmp/rollboard-server-XXXXXX)
cd "$ROOT/backend"
go build -o "$SERVER_BIN" ./cmd/server/
cd "$ROOT"

# Run directly (no go wrapper) so kill works properly
ROLLBOARD_DATABASE_URL="$DATABASE_URL" ROLLBOARD_REDIS_URL="$REDIS_URL" "$SERVER_BIN" -addr "$ADDR" &
BACKEND_PID=$!

# Wait for server to start
for i in $(seq 1 15); do
  if curl -s -o /dev/null -w "%{http_code}" "http://$ADDR/api/health" 2>/dev/null | grep -q 200; then
    break
  fi
  sleep 1
done

# Final health check
HEALTH=$(curl -s -o /dev/null -w "%{http_code}" "http://$ADDR/api/health" 2>/dev/null || echo "000")
if [ "$HEALTH" != "200" ]; then
  echo "  FAIL: backend did not start (health check returned $HEALTH)"
  exit 1
fi
echo "  Backend is healthy."

docker compose --project-directory "$ROOT" exec -T postgres \
  psql -U rollboard -d rollboard_test -c 'TRUNCATE sessions, games CASCADE' >/dev/null

curl --fail --silent --show-error --cookie "$COOKIE_JAR" --cookie-jar "$COOKIE_JAR" \
  --request POST --header 'Content-Type: application/json' --data '{"displayName":"Demo validator"}' "http://$ADDR/api/auth/guest" >/dev/null
account_email="demo-validator-${RANDOM}-${RANDOM}@example.com"
curl --fail --silent --show-error --cookie "$COOKIE_JAR" --cookie-jar "$COOKIE_JAR" \
  --request POST --header 'Content-Type: application/json' --data "{\"email\":\"$account_email\",\"displayName\":\"Demo validator\",\"password\":\"correct-horse-battery-staple\"}" "http://$ADDR/api/auth/register" >/dev/null
CSRF_TOKEN="$(awk '$6 == "rollboard_csrf" {print $7}' "$COOKIE_JAR")"
if [ -z "$CSRF_TOKEN" ]; then
  echo "  FAIL: registration did not establish a CSRF token"
  exit 1
fi

# --- Helper: POST a game definition and validate it ---
validate_demo() {
  local name="$1"
  local payload="$2"
  local game_id="$3"

  # Create game
  local created_game
  if ! created_game=$(curl --fail --silent --show-error --cookie "$COOKIE_JAR" -H "X-CSRF-Token: $CSRF_TOKEN" \
    -X POST -H "Content-Type: application/json" -d "$payload" "http://$ADDR/api/games"); then
    echo "  FAIL: $name — create request failed"
    return 1
  fi
  game_id="$(printf '%s' "$created_game" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')"
  if [ -z "$game_id" ]; then
    echo "  FAIL: $name — create response has no game ID"
    return 1
  fi

  # Validate using the known game ID. Validation is owner-scoped, so the
  # session cookie and CSRF token are required.
  local validate_res
  validate_res=$(curl -s --cookie "$COOKIE_JAR" -H "X-CSRF-Token: $CSRF_TOKEN" \
    -X POST "http://$ADDR/api/games/$game_id/validate" 2>/dev/null || echo '{"valid":false}')
  if echo "$validate_res" | grep -q '"valid":true'; then
    echo "  PASS: $name"
    return 0
  else
    local errors
    errors=$(echo "$validate_res" | grep -o '"errors":\[[^]]*\]' || echo "unknown errors")
    echo "  FAIL: $name — validation failed: $errors"
    return 1
  fi
}

# ─── Mini-Monopoly Demo ──────────────────────────────────────────────
# Data must match frontend/src/lib/defaults.ts → createMiniMonopolyDemo()
MINI_MONOPOLY='{
  "id": "validate-mini-monopoly",
  "title": "Mini-Monopoly Validate",
  "version": 1,
  "board": {
    "width": 864,
    "height": 768,
    "cellSize": 96,
    "cells": [
      {"id":"cell_1","title":"Start","type":"start","x":0,"y":576,"visual":{"baseColor":"#4CAF50","baseImage":""},"fields":{},"onLand":[]},
      {"id":"cell_2","title":"Street A","type":"property","x":192,"y":576,"visual":{"baseColor":"#E3F2FD","baseImage":""},"fields":{"cost":100,"rent":20,"colorGroup":"brown"},"onLand":[{"type":"if_cell_unowned","then":[{"type":"offer_choice","title":"Buy this property?","options":[{"id":"buy_property","title":"Buy","then":[{"type":"lose_resource","resource":"money","amountField":"cost"},{"type":"set_cell_owner","target":"current"}]},{"id":"skip_purchase","title":"Don'\''t Buy","then":[]}]}],"else":[{"type":"if_cell_owned_by_other","then":[{"type":"transfer_resource","resource":"money","amountField":"rent","target":"owner"}]}]}]},
      {"id":"cell_3","title":"Bonus +50","type":"bonus","x":384,"y":576,"visual":{"baseColor":"#C8E6C9","baseImage":""},"fields":{"amount":50},"onLand":[{"type":"gain_resource","resource":"money","amountField":"amount"}]},
      {"id":"cell_4","title":"Street B","type":"property","x":576,"y":576,"visual":{"baseColor":"#E3F2FD","baseImage":""},"fields":{"cost":120,"rent":25,"colorGroup":"brown"},"onLand":[{"type":"if_cell_unowned","then":[{"type":"offer_choice","title":"Buy this property?","options":[{"id":"buy_property","title":"Buy","then":[{"type":"lose_resource","resource":"money","amountField":"cost"},{"type":"set_cell_owner","target":"current"}]},{"id":"skip_purchase","title":"Don'\''t Buy","then":[]}]}],"else":[{"type":"if_cell_owned_by_other","then":[{"type":"transfer_resource","resource":"money","amountField":"rent","target":"owner"}]}]}]},
      {"id":"cell_5","title":"Penalty -40","type":"penalty","x":0,"y":480,"visual":{"baseColor":"#FFCDD2","baseImage":""},"fields":{"amount":40},"onLand":[{"type":"lose_resource","resource":"money","amountField":"amount"}]},
      {"id":"cell_6","title":"Street C","type":"property","x":192,"y":480,"visual":{"baseColor":"#FFE0B2","baseImage":""},"fields":{"cost":150,"rent":30,"colorGroup":"orange"},"onLand":[{"type":"if_cell_unowned","then":[{"type":"offer_choice","title":"Buy this property?","options":[{"id":"buy_property","title":"Buy","then":[{"type":"lose_resource","resource":"money","amountField":"cost"},{"type":"set_cell_owner","target":"current"}]},{"id":"skip_purchase","title":"Don'\''t Buy","then":[]}]}],"else":[{"type":"if_cell_owned_by_other","then":[{"type":"transfer_resource","resource":"money","amountField":"rent","target":"owner"}]}]}]},
      {"id":"cell_7","title":"Empty","type":"empty","x":384,"y":480,"visual":{"baseColor":"#F5F5F5","baseImage":""},"fields":{},"onLand":[]},
      {"id":"cell_8","title":"Street D","type":"property","x":576,"y":480,"visual":{"baseColor":"#FFE0B2","baseImage":""},"fields":{"cost":180,"rent":35,"colorGroup":"orange"},"onLand":[{"type":"if_cell_unowned","then":[{"type":"offer_choice","title":"Buy this property?","options":[{"id":"buy_property","title":"Buy","then":[{"type":"lose_resource","resource":"money","amountField":"cost"},{"type":"set_cell_owner","target":"current"}]},{"id":"skip_purchase","title":"Don'\''t Buy","then":[]}]}],"else":[{"type":"if_cell_owned_by_other","then":[{"type":"transfer_resource","resource":"money","amountField":"rent","target":"owner"}]}]}]},
      {"id":"cell_9","title":"Bonus +70","type":"bonus","x":0,"y":384,"visual":{"baseColor":"#C8E6C9","baseImage":""},"fields":{"amount":70},"onLand":[{"type":"gain_resource","resource":"money","amountField":"amount"}]},
      {"id":"cell_10","title":"Street E","type":"property","x":192,"y":384,"visual":{"baseColor":"#FFF9C4","baseImage":""},"fields":{"cost":200,"rent":40,"colorGroup":"yellow"},"onLand":[{"type":"if_cell_unowned","then":[{"type":"offer_choice","title":"Buy this property?","options":[{"id":"buy_property","title":"Buy","then":[{"type":"lose_resource","resource":"money","amountField":"cost"},{"type":"set_cell_owner","target":"current"}]},{"id":"skip_purchase","title":"Don'\''t Buy","then":[]}]}],"else":[{"type":"if_cell_owned_by_other","then":[{"type":"transfer_resource","resource":"money","amountField":"rent","target":"owner"}]}]}]},
      {"id":"cell_11","title":"Penalty -60","type":"penalty","x":384,"y":384,"visual":{"baseColor":"#FFCDD2","baseImage":""},"fields":{"amount":60},"onLand":[{"type":"lose_resource","resource":"money","amountField":"amount"}]},
      {"id":"cell_12","title":"Street F","type":"property","x":576,"y":384,"visual":{"baseColor":"#FFF9C4","baseImage":""},"fields":{"cost":220,"rent":45,"colorGroup":"yellow"},"onLand":[{"type":"if_cell_unowned","then":[{"type":"offer_choice","title":"Buy this property?","options":[{"id":"buy_property","title":"Buy","then":[{"type":"lose_resource","resource":"money","amountField":"cost"},{"type":"set_cell_owner","target":"current"}]},{"id":"skip_purchase","title":"Don'\''t Buy","then":[]}]}],"else":[{"type":"if_cell_owned_by_other","then":[{"type":"transfer_resource","resource":"money","amountField":"rent","target":"owner"}]}]}]},
      {"id":"cell_13","title":"Empty","type":"empty","x":0,"y":288,"visual":{"baseColor":"#F5F5F5","baseImage":""},"fields":{},"onLand":[]},
      {"id":"cell_14","title":"Street G","type":"property","x":192,"y":288,"visual":{"baseColor":"#E1BEE7","baseImage":""},"fields":{"cost":240,"rent":50,"colorGroup":"purple"},"onLand":[{"type":"if_cell_unowned","then":[{"type":"offer_choice","title":"Buy this property?","options":[{"id":"buy_property","title":"Buy","then":[{"type":"lose_resource","resource":"money","amountField":"cost"},{"type":"set_cell_owner","target":"current"}]},{"id":"skip_purchase","title":"Don'\''t Buy","then":[]}]}],"else":[{"type":"if_cell_owned_by_other","then":[{"type":"transfer_resource","resource":"money","amountField":"rent","target":"owner"}]}]}]},
      {"id":"cell_15","title":"Bonus +100","type":"bonus","x":384,"y":288,"visual":{"baseColor":"#C8E6C9","baseImage":""},"fields":{"amount":100},"onLand":[{"type":"gain_resource","resource":"money","amountField":"amount"}]},
      {"id":"cell_16","title":"Street H","type":"property","x":576,"y":288,"visual":{"baseColor":"#E1BEE7","baseImage":""},"fields":{"cost":260,"rent":55,"colorGroup":"purple"},"onLand":[{"type":"if_cell_unowned","then":[{"type":"offer_choice","title":"Buy this property?","options":[{"id":"buy_property","title":"Buy","then":[{"type":"lose_resource","resource":"money","amountField":"cost"},{"type":"set_cell_owner","target":"current"}]},{"id":"skip_purchase","title":"Don'\''t Buy","then":[]}]}],"else":[{"type":"if_cell_owned_by_other","then":[{"type":"transfer_resource","resource":"money","amountField":"rent","target":"owner"}]}]}]}
    ],
    "edges": [
      {"id":"e1","from":"cell_1","to":"cell_2","condition":{"type":"always"}},
      {"id":"e2","from":"cell_2","to":"cell_3","condition":{"type":"always"}},
      {"id":"e3","from":"cell_3","to":"cell_4","condition":{"type":"always"}},
      {"id":"e4","from":"cell_4","to":"cell_5","condition":{"type":"always"}},
      {"id":"e5","from":"cell_5","to":"cell_6","condition":{"type":"always"}},
      {"id":"e6","from":"cell_6","to":"cell_7","condition":{"type":"always"}},
      {"id":"e7","from":"cell_7","to":"cell_8","condition":{"type":"always"}},
      {"id":"e8","from":"cell_8","to":"cell_9","condition":{"type":"always"}},
      {"id":"e9","from":"cell_9","to":"cell_10","condition":{"type":"always"}},
      {"id":"e10","from":"cell_10","to":"cell_11","condition":{"type":"always"}},
      {"id":"e11","from":"cell_11","to":"cell_12","condition":{"type":"always"}},
      {"id":"e12","from":"cell_12","to":"cell_13","condition":{"type":"always"}},
      {"id":"e13","from":"cell_13","to":"cell_14","condition":{"type":"always"}},
      {"id":"e14","from":"cell_14","to":"cell_15","condition":{"type":"always"}},
      {"id":"e15","from":"cell_15","to":"cell_16","condition":{"type":"always"}},
      {"id":"e16","from":"cell_16","to":"cell_1","condition":{"type":"always"}}
    ]
  },
  "rules": {
    "dice": {"count":1,"sides":6},
    "resources": {
      "money": {"initial":500}
    },
    "cellTypes": {
      "start": {"title":"Start","fields":{}},
      "property": {"title":"Property","fields":{"cost":{"type":"number","label":"Cost","default":100},"rent":{"type":"number","label":"Rent","default":20},"colorGroup":{"type":"string","label":"Color Group","default":""}}},
      "bonus": {"title":"Bonus","fields":{"amount":{"type":"number","label":"Amount","default":50}}},
      "penalty": {"title":"Penalty","fields":{"amount":{"type":"number","label":"Amount","default":40}}},
      "empty": {"title":"Empty","fields":{}}
    },
    "startBonus": 100,
    "startBonusResource": "money"
  }
}'

# ─── Dungeon Race Demo ──────────────────────────────────────────────
# Data must match frontend/src/lib/defaults.ts → createDungeonRaceDemo()
DUNGEON_RACE='{
  "id": "validate-dungeon-race",
  "title": "Dungeon Race Validate",
  "version": 1,
  "board": {
    "width": 1056,
    "height": 384,
    "cellSize": 96,
    "cells": [
      {"id":"start","title":"Start","type":"start","x":0,"y":96,"visual":{"baseColor":"#4CAF50","baseImage":""},"fields":{},"onLand":[]},
      {"id":"trap","title":"Trap -2 HP","type":"trap","x":192,"y":96,"visual":{"baseColor":"#FFCDD2","baseImage":""},"fields":{},"onLand":[{"type":"lose_resource","resource":"health","amount":2}]},
      {"id":"treasure","title":"Treasure +5 Gold","type":"treasure","x":384,"y":96,"visual":{"baseColor":"#FFF9C4","baseImage":""},"fields":{},"onLand":[{"type":"gain_resource","resource":"gold","amount":5}]},
      {"id":"key","title":"Key +1","type":"key","x":576,"y":96,"visual":{"baseColor":"#E1BEE7","baseImage":""},"fields":{},"onLand":[{"type":"gain_resource","resource":"keys","amount":1}]},
      {"id":"heal","title":"Heal +2 HP","type":"heal","x":768,"y":96,"visual":{"baseColor":"#C8E6C9","baseImage":""},"fields":{},"onLand":[{"type":"gain_resource","resource":"health","amount":2}]},
      {"id":"finish","title":"Finish!","type":"finish","x":960,"y":96,"visual":{"baseColor":"#FFD700","baseImage":""},"fields":{},"onLand":[{"type":"finish_game"}]}
    ],
    "edges": [
      {"id":"e1","from":"start","to":"trap","condition":{"type":"always"}},
      {"id":"e2","from":"trap","to":"treasure","condition":{"type":"always"}},
      {"id":"e3","from":"treasure","to":"key","condition":{"type":"always"}},
      {"id":"e4","from":"key","to":"heal","condition":{"type":"always"}},
      {"id":"e5","from":"heal","to":"finish","condition":{"type":"always"}}
    ]
  },
  "rules": {
    "dice": {"count":1,"sides":6},
    "resources": {
      "health": {"initial":10,"min":0,"max":10},
      "gold": {"initial":0,"min":0},
      "keys": {"initial":0,"min":0}
    },
    "cellTypes": {
      "start": {"title":"Start","fields":{}},
      "trap": {"title":"Trap","fields":{"amount":{"type":"number","label":"Damage","default":2}}},
      "treasure": {"title":"Treasure","fields":{"amount":{"type":"number","label":"Gold","default":5}}},
      "key": {"title":"Key","fields":{"amount":{"type":"number","label":"Keys","default":1}}},
      "heal": {"title":"Heal","fields":{"amount":{"type":"number","label":"HP","default":2}}},
      "finish": {"title":"Finish","fields":{}}
    },
    "startBonus": 0,
    "startBonusResource": ""
  }
}'

# ─── Run validations ────────────────────────────────────────────────
echo ""

if validate_demo "Mini-Monopoly" "$MINI_MONOPOLY" "validate-mini-monopoly"; then
  PASS=$((PASS + 1))
else
  FAIL=$((FAIL + 1))
fi

if validate_demo "Dungeon Race" "$DUNGEON_RACE" "validate-dungeon-race"; then
  PASS=$((PASS + 1))
else
  FAIL=$((FAIL + 1))
fi

echo ""
echo "=== Results: $PASS passed, $FAIL failed ==="
if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
