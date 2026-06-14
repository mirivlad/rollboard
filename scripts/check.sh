#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"

echo "=== Running backend checks ==="
cd "$ROOT/backend"
go vet ./...
go test ./... -count=1

echo ""
echo "=== Running frontend checks ==="
cd "$ROOT/frontend"
if [ -d node_modules ]; then
  echo "--- svelte-check ---"
  npx svelte-check --tsconfig ./tsconfig.json 2>&1 | tail -1
fi
echo "--- vite build ---"
npx vite build 2>&1 | tail -1

echo ""
echo "=== Validating demo definitions ==="
cd "$ROOT"
./scripts/validate-demos.sh

echo ""
echo "All checks passed!"
