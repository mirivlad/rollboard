#!/usr/bin/env bash
# shutdown-smoke.sh — start the server, stop it the way a container runtime
# does, and check it goes down on purpose rather than being killed mid-request.
#
# The server used to run on a bare http.ListenAndServe with no signal handling
# at all, so SIGTERM took the process out immediately and cut whatever was in
# flight. This is the check that it no longer does.
#
# Usage: ./scripts/shutdown-smoke.sh

set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PORT=$(( (RANDOM % 1000) + 9200 ))
ADDR="127.0.0.1:${PORT}"
DATABASE_URL="${ROLLBOARD_TEST_DATABASE_URL:-postgres://rollboard:rollboard@127.0.0.1:5432/rollboard_test?sslmode=disable}"
REDIS_URL="${ROLLBOARD_TEST_REDIS_URL:-redis://127.0.0.1:6379/4}"
LOG="$(mktemp)"
BINARY="$(mktemp /tmp/rollboard-shutdown-XXXXXX)"
PID=""

cleanup() {
  [ -n "$PID" ] && kill -9 "$PID" 2>/dev/null || true
  rm -f "$LOG" "$BINARY" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

echo "=== Shutdown smoke test ==="
# The realtime check before this one tears the stack down when it finishes, so
# this brings up what it needs rather than assuming.
if [ "${ROLLBOARD_SKIP_TEST_SERVICES:-0}" != "1" ]; then
  "$ROOT/scripts/test-services.sh"
fi
(cd "$ROOT/backend" && go build -o "$BINARY" ./cmd/server)

ROLLBOARD_DATABASE_URL="$DATABASE_URL" ROLLBOARD_REDIS_URL="$REDIS_URL" \
  "$BINARY" -addr "$ADDR" > "$LOG" 2>&1 &
PID=$!

for _ in $(seq 1 30); do
  if curl --noproxy '*' --fail --silent "http://$ADDR/api/health" >/dev/null 2>&1; then break; fi
  sleep 1
done
if ! curl --noproxy '*' --fail --silent "http://$ADDR/api/health" >/dev/null 2>&1; then
  echo "  FAIL: the server never became healthy" >&2
  cat "$LOG" >&2
  exit 1
fi
echo "  Server is healthy."

kill -TERM "$PID"
started=$(date +%s)
status=0
wait "$PID" || status=$?
elapsed=$(( $(date +%s) - started ))
PID=""

if [ "$status" -ne 0 ]; then
  echo "  FAIL: exit status $status after SIGTERM" >&2
  cat "$LOG" >&2
  exit 1
fi
if ! grep -q "shutting down" "$LOG" || ! grep -q "shutdown complete" "$LOG"; then
  echo "  FAIL: the server did not report a graceful shutdown" >&2
  cat "$LOG" >&2
  exit 1
fi
if [ "$elapsed" -gt 20 ]; then
  echo "  FAIL: shutdown took ${elapsed}s" >&2
  exit 1
fi

echo "  PASS: stopped gracefully in ${elapsed}s."
