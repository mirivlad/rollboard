#!/usr/bin/env bash
set -euo pipefail

for pid_file in /tmp/rollboard-dev-backend.pid /tmp/rollboard-dev-frontend.pid; do
  if [ -f "$pid_file" ]; then
    pid="$(<"$pid_file")"
    kill "$pid" 2>/dev/null || true
    rm -f "$pid_file"
  fi
done
