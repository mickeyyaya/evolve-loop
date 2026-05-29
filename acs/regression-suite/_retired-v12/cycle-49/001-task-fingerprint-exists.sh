#!/usr/bin/env bash
# ACS predicate: task-fingerprint.sh exists and is executable
# metadata: cycle=49 slug=task-fingerprint-exists

set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
SCRIPT="$WORKTREE/scripts/utility/task-fingerprint.sh"
[ -f "$SCRIPT" ] || { echo "FAIL: $SCRIPT not found"; exit 1; }
[ -x "$SCRIPT" ] || { echo "FAIL: $SCRIPT not executable"; exit 1; }
echo "GREEN: task-fingerprint.sh exists and is executable"
