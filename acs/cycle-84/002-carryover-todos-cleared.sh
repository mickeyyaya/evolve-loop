#!/usr/bin/env bash
# AC-ID: cycle-84-002
# Verify state.json:carryoverTodos is an empty array.
set -uo pipefail
REPO_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
STATE="$REPO_ROOT/.evolve/state.json"
[ -f "$STATE" ] || { echo "RED cycle-84-002: $STATE missing"; exit 1; }
count=$(jq '.carryoverTodos | length' "$STATE" 2>/dev/null)
if [ "$count" != "0" ]; then
    echo "RED cycle-84-002: carryoverTodos has $count items (expected 0)"
    exit 1
fi
echo "GREEN cycle-84-002: carryoverTodos is empty"
exit 0
