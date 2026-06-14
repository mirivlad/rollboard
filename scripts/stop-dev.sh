#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"

echo "=== Rollboard Stop Dev ==="

# Kill rollboard-server processes
for pid_file in /tmp/rollboard-server.pid; do
  if [ -f "$pid_file" ]; then
    PID=$(cat "$pid_file" 2>/dev/null || true)
    if [ -n "$PID" ] && kill -0 "$PID" 2>/dev/null; then
      echo "  Killing rollboard-server (PID $PID)..."
      kill "$PID" 2>/dev/null || true
    fi
    rm -f "$pid_file"
  fi
done

# Find and kill remaining processes
pkill -f "rollboard-server" 2>/dev/null || true
pkill -f "vite" 2>/dev/null || true
pkill -f "esbuild" 2>/dev/null || true

# Check if ports are now free
sleep 1
PORTS="8080 5173"
for PORT in $PORTS; do
  if ss -ltn "sport = :$PORT" 2>/dev/null | grep -q ":$PORT"; then
    PID=$(ss -ltnp "sport = :$PORT" 2>/dev/null | grep -oP 'pid=\K[0-9]+' || true)
    if [ -n "$PID" ]; then
      echo "  Port $PORT still in use by PID $PID, killing..."
      kill "$PID" 2>/dev/null || true
      sleep 1
    fi
  fi
done

echo "  Done."
