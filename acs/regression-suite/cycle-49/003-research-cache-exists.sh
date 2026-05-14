#!/usr/bin/env bash
# ACS predicate: research-cache.sh exists, is executable, and returns 50 when disabled
# metadata: cycle=49 slug=research-cache-exists

set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
SCRIPT="$WORKTREE/scripts/utility/research-cache.sh"
[ -f "$SCRIPT" ] || { echo "FAIL: $SCRIPT not found"; exit 1; }
[ -x "$SCRIPT" ] || { echo "FAIL: $SCRIPT not executable"; exit 1; }

# Feature gate: exits 50 when EVOLVE_RESEARCH_CACHE_ENABLED unset
bash "$SCRIPT" check "test-task" 2>/dev/null
rc=$?
[ "$rc" -eq 50 ] || { echo "FAIL: expected exit 50 (DISABLED), got $rc"; exit 1; }
echo "GREEN: research-cache.sh exists and returns DISABLED (50) by default"
