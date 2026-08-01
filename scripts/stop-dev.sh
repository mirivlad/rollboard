#!/usr/bin/env bash
set -euo pipefail

for pid_file in /tmp/rollboard-dev-backend.pid /tmp/rollboard-dev-frontend.pid; do
  if [ -f "$pid_file" ]; then
    pid="$(<"$pid_file")"
    if [[ "$pid" =~ ^[0-9]+$ ]]; then
      pgid="$(ps -o pgid= -p "$pid" 2>/dev/null | tr -d ' ' || true)"
      if [ -n "$pgid" ]; then
        kill -- "-$pgid" 2>/dev/null || true
      fi
    fi
    rm -f "$pid_file"
  fi
done
