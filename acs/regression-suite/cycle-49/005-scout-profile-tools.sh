#!/usr/bin/env bash
# ACS predicate: scout.json allowed_tools includes research-cache and task-fingerprint
# metadata: cycle=49 slug=scout-profile-tools

set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
PROFILE="$WORKTREE/.evolve/profiles/scout.json"
[ -f "$PROFILE" ] || { echo "FAIL: $PROFILE not found"; exit 1; }
command -v jq >/dev/null 2>&1 || { echo "FAIL: jq required"; exit 1; }

tools=$(jq -r '.allowed_tools[]' "$PROFILE" 2>/dev/null)
echo "$tools" | grep -q "research-cache.sh" || { echo "FAIL: research-cache.sh not in scout allowed_tools"; exit 1; }
echo "$tools" | grep -q "task-fingerprint.sh" || { echo "FAIL: task-fingerprint.sh not in scout allowed_tools"; exit 1; }
echo "GREEN: scout.json includes research-cache.sh and task-fingerprint.sh in allowed_tools"
