#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
PROJECT_NAME="rollboard-smoke"

cleanup() {
  docker compose --project-name "$PROJECT_NAME" --project-directory "$ROOT_DIR" down --volumes --remove-orphans >/dev/null 2>&1 || true
}

trap cleanup EXIT INT TERM

cd "$ROOT_DIR"
docker compose --project-name "$PROJECT_NAME" up --build --detach

for attempt in $(seq 1 45); do
  if curl --noproxy '*' --fail --silent --show-error http://127.0.0.1:8080/readyz | grep --quiet '"status":"ready"'; then
    break
  fi
  if [ "$attempt" -eq 45 ]; then
    docker compose --project-name "$PROJECT_NAME" logs
    exit 1
  fi
  sleep 1
done

create_body='{"id":"docker-smoke-game","title":"Docker Smoke","version":1,"board":{"width":96,"height":96,"cellSize":96,"cells":[{"id":"start","title":"Start","type":"start","x":0,"y":0,"visual":{"baseColor":"#4caf50"},"fields":{}}],"edges":[]},"rules":{"dice":{"count":1,"sides":6},"resources":{},"cellTypes":{},"startBonus":0,"startBonusResource":""}}'
curl --noproxy '*' --fail --silent --show-error --request POST --header 'Content-Type: application/json' --data "$create_body" http://127.0.0.1:8080/api/games >/dev/null

docker compose --project-name "$PROJECT_NAME" restart app
for attempt in $(seq 1 45); do
  if curl --noproxy '*' --fail --silent --show-error http://127.0.0.1:8080/readyz | grep --quiet '"status":"ready"'; then
    break
  fi
  if [ "$attempt" -eq 45 ]; then
    docker compose --project-name "$PROJECT_NAME" logs
    exit 1
  fi
  sleep 1
done

curl --noproxy '*' --fail --silent --show-error http://127.0.0.1:8080/api/games/docker-smoke-game | grep --quiet '"id":"docker-smoke-game"'
