#!/usr/bin/env bash
# test.sh — run the full Go suite with integration tests enabled.
#
# The integration tests skip themselves unless ROLLBOARD_TEST_DATABASE_URL and
# ROLLBOARD_TEST_REDIS_URL are set. Setting them here is the difference between
# a green run that proves something and a green run that proved nothing.
#
# Set ROLLBOARD_SKIP_TEST_SERVICES=1 when PostgreSQL and Redis are already
# provided by the environment, as they are in CI.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"

if [ "${ROLLBOARD_SKIP_TEST_SERVICES:-0}" != "1" ]; then
  "$ROOT_DIR/scripts/test-services.sh"
fi

# Integration tests truncate tables, so they must never point at the
# development database.
export ROLLBOARD_TEST_DATABASE_URL="${ROLLBOARD_TEST_DATABASE_URL:-postgres://rollboard:rollboard@127.0.0.1:5432/rollboard_test?sslmode=disable}"
export ROLLBOARD_TEST_REDIS_URL="${ROLLBOARD_TEST_REDIS_URL:-redis://127.0.0.1:6379/1}"

case "$ROLLBOARD_TEST_DATABASE_URL" in
  */rollboard_test*) ;;
  *)
    echo "Refusing to run: ROLLBOARD_TEST_DATABASE_URL must target the rollboard_test database." >&2
    echo "  got: $ROLLBOARD_TEST_DATABASE_URL" >&2
    exit 1
    ;;
esac

cd "$ROOT_DIR/backend"

# -p 1 because every integration package truncates the one shared test
# database and serialises on the same advisory lock; running them concurrently
# only adds lock queueing. -timeout keeps a wedged test from burning ten
# minutes before it reports.
go test ./... -count=1 -p 1 -timeout 300s "$@"
