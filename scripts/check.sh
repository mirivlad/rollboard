#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"

echo "=== Checking Go formatting ==="
cd "$ROOT/backend"
unformatted="$(gofmt -l ./cmd ./internal)"
if [ -n "$unformatted" ]; then
  echo "These files are not gofmt-clean:" >&2
  echo "$unformatted" >&2
  echo "Run 'make fmt'." >&2
  exit 1
fi
echo "  gofmt clean."

echo ""
echo "=== Running go vet ==="
go vet ./...

echo ""
echo "=== Running backend tests (integration tests enabled) ==="
"$ROOT/scripts/test.sh"

echo ""
echo "=== Verifying Redis realtime fan-out ==="
"$ROOT/scripts/test-realtime-redis.sh"

echo ""
"$ROOT/scripts/shutdown-smoke.sh"

echo ""
echo "=== Running frontend checks ==="
cd "$ROOT/frontend"
if [ ! -d node_modules ]; then
  npm ci
fi
echo "--- svelte-check ---"
npx svelte-check --tsconfig ./tsconfig.json
echo "--- vitest ---"
npm test
echo "--- vite build ---"
npx vite build

echo ""
cd "$ROOT"
./scripts/validate-templates.sh

echo ""
echo "All checks passed!"
