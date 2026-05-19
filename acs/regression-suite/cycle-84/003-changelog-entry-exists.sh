#!/usr/bin/env bash
# AC-ID: cycle-84-003
# Verify CHANGELOG.md contains "Cycle 84" entry.
set -uo pipefail
REPO_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
# Check active worktree first (CHANGELOG is a worktree change, not yet on main)
WORKTREE=$(cycle-state.sh get active_worktree 2>/dev/null | tr -d '[:space:]' || echo "")
CHANGELOG_PATH=""
if [ -n "$WORKTREE" ] && [ -f "$WORKTREE/CHANGELOG.md" ]; then
    CHANGELOG_PATH="$WORKTREE/CHANGELOG.md"
else
    CHANGELOG_PATH="$REPO_ROOT/CHANGELOG.md"
fi
[ -f "$CHANGELOG_PATH" ] || { echo "RED cycle-84-003: CHANGELOG.md missing"; exit 1; }
grep -qi "Cycle 84" "$CHANGELOG_PATH" || { echo "RED cycle-84-003: 'Cycle 84' not found in $CHANGELOG_PATH"; exit 1; }
echo "GREEN cycle-84-003: 'Cycle 84' found in $CHANGELOG_PATH"
exit 0
