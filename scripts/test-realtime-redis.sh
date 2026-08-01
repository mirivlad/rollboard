#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"

cleanup() {
  docker compose --project-directory "$ROOT_DIR" down --remove-orphans >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

docker compose --project-directory "$ROOT_DIR" up --detach redis >/dev/null
for attempt in $(seq 1 30); do
  if docker compose --project-directory "$ROOT_DIR" exec -T redis redis-cli ping >/dev/null 2>&1; then
    break
  fi
  if [ "$attempt" -eq 30 ]; then
    echo "Redis did not become ready" >&2
    exit 1
  fi
  sleep 1
done

cd "$ROOT_DIR/backend"
ROLLBOARD_TEST_REDIS_URL="redis://127.0.0.1:6379/0" \
  go test ./internal/realtime -run TestHubsBroadcastTransitionsThroughRedis -count=1 -timeout 30s
