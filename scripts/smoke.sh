#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
ADDR="${ROLLBOARD_SMOKE_ADDR:-127.0.0.1:18091}"
DATABASE_URL="postgres://rollboard:rollboard@127.0.0.1:5432/rollboard_test?sslmode=disable"
SERVER_PID=""

cleanup() {
  if [ -n "$SERVER_PID" ]; then
    kill "$SERVER_PID" 2>/dev/null || true
    wait "$SERVER_PID" 2>/dev/null || true
  fi

  docker compose --project-directory "$ROOT_DIR" down --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

cd "$ROOT_DIR"
docker compose up --detach postgres redis >/dev/null
for attempt in $(seq 1 30); do
  if docker compose exec -T postgres pg_isready -U rollboard -d rollboard_test >/dev/null 2>&1; then break; fi
  if [ "$attempt" -eq 30 ]; then exit 1; fi
  sleep 1
done

(cd backend && go build -o /tmp/rollboard-server-smoke ./cmd/server)
ROLLBOARD_ADDR="$ADDR" ROLLBOARD_DATABASE_URL="$DATABASE_URL" /tmp/rollboard-server-smoke > /tmp/rollboard-server.log 2>&1 &
SERVER_PID=$!
for attempt in $(seq 1 30); do
  if curl --noproxy '*' --fail --silent "http://$ADDR/readyz" >/dev/null; then break; fi
  if [ "$attempt" -eq 30 ]; then cat /tmp/rollboard-server.log; exit 1; fi
  sleep 1
done

game='{"id":"smoke-test-game","title":"Smoke Test","version":1,"board":{"width":96,"height":96,"cellSize":96,"cells":[{"id":"start","title":"Start","type":"start","x":0,"y":0,"visual":{"baseColor":"#4caf50"},"fields":{}}],"edges":[]},"rules":{"dice":{"count":1,"sides":6},"resources":{},"cellTypes":{},"startBonus":0,"startBonusResource":""}}'
curl --noproxy '*' --fail --silent --show-error --request POST --header 'Content-Type: application/json' --data "$game" "http://$ADDR/api/games" >/dev/null || true
curl --noproxy '*' --fail --silent --show-error "http://$ADDR/api/games/smoke-test-game" | grep --quiet '"id":"smoke-test-game"'
curl --noproxy '*' --fail --silent --show-error --request POST "http://$ADDR/api/games/smoke-test-game/validate" | grep --quiet '"valid":true'
