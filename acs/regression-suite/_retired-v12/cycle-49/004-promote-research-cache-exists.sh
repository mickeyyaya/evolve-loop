#!/usr/bin/env bash
# ACS predicate: promote-research-cache.sh exists, is executable, and NOOPs when disabled
# metadata: cycle=49 slug=promote-research-cache-exists

set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
SCRIPT="$WORKTREE/scripts/lifecycle/promote-research-cache.sh"
[ -f "$SCRIPT" ] || { echo "FAIL: $SCRIPT not found"; exit 1; }
[ -x "$SCRIPT" ] || { echo "FAIL: $SCRIPT not executable"; exit 1; }

# NOOP when disabled
out=$(bash "$SCRIPT" 49 "/tmp/noop-workspace-$$" 2>&1)
rc=$?
[ "$rc" -eq 0 ] || { echo "FAIL: expected exit 0 when disabled, got $rc"; exit 1; }
echo "$out" | grep -q "NOOP" || { echo "FAIL: expected NOOP message, got: $out"; exit 1; }
echo "GREEN: promote-research-cache.sh exists and NOOPs when disabled"
