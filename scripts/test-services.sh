#!/usr/bin/env bash
# test-services.sh — bring up the PostgreSQL and Redis services the integration
# tests need, and wait until both are actually answering.
#
# Sourced or executed. Safe to call repeatedly; Compose reuses running services.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

docker compose --project-directory "$ROOT_DIR" up --detach postgres redis >/dev/null

for attempt in $(seq 1 30); do
  if docker compose --project-directory "$ROOT_DIR" exec -T postgres pg_isready -U rollboard -d rollboard_test >/dev/null 2>&1; then
    break
  fi
  if [ "$attempt" -eq 30 ]; then
    echo "PostgreSQL test database did not become ready" >&2
    exit 1
  fi
  sleep 1
done

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
