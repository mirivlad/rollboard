#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/../frontend"

echo "Starting frontend on http://127.0.0.1:5173"
npx vite --host 127.0.0.1
