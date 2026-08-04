#!/usr/bin/env bash
# validate-templates.sh — check every bundled template against the real
# publication rules.
#
# The templates are what a new author starts from, so a template the validator
# would refuse is a broken first experience. This runs the same
# game.ValidateDefinition the API calls, with no server and no database.
#
# Usage: ./scripts/validate-templates.sh

set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"

echo "=== Validating bundled templates ==="
node "$ROOT/frontend/scripts/dump-templates.mjs" | (cd "$ROOT/backend" && go run ./cmd/validate)
