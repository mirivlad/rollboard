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
npx vite build 2>&1 | tail -3
echo ""
echo "All checks passed!"
