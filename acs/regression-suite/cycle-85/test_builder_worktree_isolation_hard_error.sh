#!/usr/bin/env bash
# ACS predicate — cycle 85
# Verifies that subagent-run.sh hard-errors (exit 1) when WORKTREE_PATH is unset
# for a worktree-aware profile, instead of warning and falling through.
set -uo pipefail

SCRIPT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}/scripts/dispatch/subagent-run.sh"

[ -f "$SCRIPT" ] || { echo "FAIL: subagent-run.sh not found at $SCRIPT" >&2; exit 1; }

# Must contain "exit 1" in the WORKTREE_PATH-unset branch (hard error)
if ! grep -qF 'exit 1' "$SCRIPT"; then
    echo "FAIL: subagent-run.sh does not contain exit 1" >&2
    exit 1
fi

# Must NOT contain the old warn+fallthrough pattern
if grep -qF '# fall through — adapter will fail loudly with its own check' "$SCRIPT"; then
    echo "FAIL: subagent-run.sh still contains warn+fallthrough comment (old behavior)" >&2
    exit 1
fi

# The hard-error message must be present
if ! grep -qF 'ERROR: profile' "$SCRIPT"; then
    echo "FAIL: subagent-run.sh missing ERROR: profile message for unset WORKTREE_PATH" >&2
    exit 1
fi

echo "PASS: subagent-run.sh hard-errors on unset WORKTREE_PATH for worktree-aware profiles"
exit 0
