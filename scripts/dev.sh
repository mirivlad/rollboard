#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"

# Check dependencies
command -v go >/dev/null 2>&1 || { echo "Error: Go is required (https://go.dev)"; exit 1; }
command -v node >/dev/null 2>&1 || { echo "Error: Node.js is required (https://nodejs.org)"; exit 1; }
command -v npm >/dev/null 2>&1 || { echo "Error: npm is required"; exit 1; }

# Ensure frontend deps
if [ ! -d "$ROOT_DIR/frontend/node_modules" ]; then
  echo "Installing frontend dependencies..."
  (cd "$ROOT_DIR/frontend" && npm install)
fi

# Ensure data dir
mkdir -p "$ROOT_DIR/data"

# Export defaults
export ROLLBOARD_ADDR="${ROLLBOARD_ADDR:-127.0.0.1:8080}"
export ROLLBOARD_DB_PATH="${ROLLBOARD_DB_PATH:-$ROOT_DIR/data/rollboard.db}"

mkdir -p "$(dirname "$ROLLBOARD_DB_PATH")"

echo "=== Rollboard Dev Server ==="
echo "  Backend: http://127.0.0.1:8080/api/health"
echo "  Frontend: http://127.0.0.1:5173"
echo ""

# Start backend in background
echo "Starting backend..."
(cd "$ROOT_DIR/backend" && go run ./cmd/server/ -addr "$ROLLBOARD_ADDR" -dsn "$ROLLBOARD_DB_PATH") &
BACKEND_PID=$!

# Start frontend
echo "Starting frontend..."
(cd "$ROOT_DIR/frontend" && npx vite --host 127.0.0.1) &
FRONTEND_PID=$!

# Trap Ctrl+C to stop both
cleanup() {
  echo ""
  echo "Shutting down..."
  kill "$BACKEND_PID" 2>/dev/null || true
  kill "$FRONTEND_PID" 2>/dev/null || true
  wait "$BACKEND_PID" 2>/dev/null || true
  wait "$FRONTEND_PID" 2>/dev/null || true
  echo "Done."
  exit 0
}
trap cleanup SIGINT SIGTERM

# Wait for either to exit
wait
