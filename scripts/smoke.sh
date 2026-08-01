#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
ADDR="${ROLLBOARD_SMOKE_ADDR:-127.0.0.1:18091}"
DATABASE_URL="postgres://rollboard:rollboard@127.0.0.1:5432/rollboard_test?sslmode=disable"
SERVER_PID=""
COOKIE_JAR="$(mktemp)"
GUEST_COOKIE_JAR="$(mktemp)"

cleanup() {
  if [ -n "$SERVER_PID" ]; then
    kill "$SERVER_PID" 2>/dev/null || true
    wait "$SERVER_PID" 2>/dev/null || true
  fi

  rm -f "$COOKIE_JAR"
  rm -f "$GUEST_COOKIE_JAR"

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
for attempt in $(seq 1 30); do
  if docker compose exec -T redis redis-cli ping >/dev/null 2>&1; then break; fi
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

game_id="smoke-test-${RANDOM}-${RANDOM}"
account_email="smoke-${RANDOM}-${RANDOM}@example.com"
game="{\"id\":\"$game_id\",\"title\":\"Smoke Test\",\"version\":1,\"board\":{\"width\":96,\"height\":96,\"cellSize\":96,\"cells\":[{\"id\":\"start\",\"title\":\"Start\",\"type\":\"start\",\"x\":0,\"y\":0,\"visual\":{\"baseColor\":\"#4caf50\"},\"fields\":{}}],\"edges\":[]},\"rules\":{\"dice\":{\"count\":1,\"sides\":6},\"resources\":{},\"cellTypes\":{\"start\":{\"title\":\"Start\",\"fields\":{}}},\"startBonus\":0,\"startBonusResource\":\"\"}}"
curl --noproxy '*' --fail --silent --show-error --cookie "$COOKIE_JAR" --cookie-jar "$COOKIE_JAR" \
  --request POST --header 'Content-Type: application/json' --data '{"displayName":"Smoke guest"}' "http://$ADDR/api/auth/guest" >/dev/null
curl --noproxy '*' --fail --silent --show-error --cookie "$COOKIE_JAR" --cookie-jar "$COOKIE_JAR" \
  --request POST --header 'Content-Type: application/json' --data "{\"email\":\"$account_email\",\"displayName\":\"Smoke author\",\"password\":\"correct-horse-battery-staple\"}" "http://$ADDR/api/auth/register" >/dev/null
csrf_token="$(awk '$6 == "rollboard_csrf" {print $7}' "$COOKIE_JAR")"
[[ -n "$csrf_token" ]]
created_game="$(curl --noproxy '*' --fail --silent --show-error --cookie "$COOKIE_JAR" --header "X-CSRF-Token: $csrf_token" --request POST --header 'Content-Type: application/json' --data "$game" "http://$ADDR/api/games")"
game_id="$(printf '%s' "$created_game" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p')"
[[ -n "$game_id" ]]
loaded_game="$(curl --noproxy '*' --fail --silent --show-error "http://$ADDR/api/games/$game_id")"
[[ "$loaded_game" == *"\"id\":\"$game_id\""* ]]
validation="$(curl --noproxy '*' --fail --silent --show-error --request POST "http://$ADDR/api/games/$game_id/validate")"
[[ "$validation" == *'"valid":true'* ]]
published="$(curl --noproxy '*' --fail --silent --show-error --cookie "$COOKIE_JAR" --header "X-CSRF-Token: $csrf_token" --request POST "http://$ADDR/api/games/$game_id/publish")"
version_id="$(printf '%s' "$published" | jq -r '.id')"
[[ -n "$version_id" ]]
created_room="$(curl --noproxy '*' --fail --silent --show-error --cookie "$COOKIE_JAR" --header "X-CSRF-Token: $csrf_token" --request POST --header 'Content-Type: application/json' --data "{\"gameVersionId\":\"$version_id\",\"title\":\"Smoke room\",\"maxPlayers\":2}" "http://$ADDR/api/rooms")"
room_id="$(printf '%s' "$created_room" | jq -r '.id')"
[[ -n "$room_id" ]]
curl --noproxy '*' --fail --silent --show-error --cookie "$GUEST_COOKIE_JAR" --cookie-jar "$GUEST_COOKIE_JAR" \
  --request POST --header 'Content-Type: application/json' --data '{"displayName":"Smoke player"}' "http://$ADDR/api/auth/guest" >/dev/null
guest_csrf_token="$(awk '$6 == "rollboard_csrf" {print $7}' "$GUEST_COOKIE_JAR")"
[[ -n "$guest_csrf_token" ]]
curl --noproxy '*' --fail --silent --show-error --cookie "$GUEST_COOKIE_JAR" --header "X-CSRF-Token: $guest_csrf_token" --request POST "http://$ADDR/api/rooms/$room_id/join" >/dev/null
room_state="$(curl --noproxy '*' --fail --silent --show-error --cookie "$COOKIE_JAR" "http://$ADDR/api/rooms/$room_id")"
[[ "$room_state" == *'"members"'* && "$room_state" == *'"Smoke player"'* ]]
