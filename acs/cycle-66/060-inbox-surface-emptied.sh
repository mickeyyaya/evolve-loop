#!/usr/bin/env bash
# ACS predicate 060 — cycle 66
# Verifies the cycle-66 worktree has zero tracked top-level *.json files
# under .evolve/inbox/. Builder stages 22 deletions before audit; ship.sh
# commits the worktree HEAD which the orchestrator merges to main.
#
# The check reads the worktree's INDEX (staged state) via
# `git -C <worktree> ls-files`, since the worktree's branch HEAD does not
# advance until ship.sh commits.
#
# AC-ID: cycle-66-060
# Description: Inbox unprocessed surface is empty in the cycle-66 worktree index
# Evidence: git -C $WORKTREE ls-files .evolve/inbox/*.json shows 0 files at top level
# Author: builder (cycle 66)
# Created: 2026-05-17T00:00:00Z
# Acceptance-of: intent.acceptance_checks "inbox/ (unprocessed) empty"
#
# metadata:
#   id: 060-inbox-surface-emptied
#   cycle: 66
#   task: c66-inbox-disposition-memo
#   severity: HIGH

set -uo pipefail

if [ -n "${EVOLVE_PROJECT_ROOT:-}" ]; then
    REPO_ROOT="$EVOLVE_PROJECT_ROOT"
else
    REPO_ROOT=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
fi
if [ -f "$REPO_ROOT/.git" ]; then
    REPO_ROOT="$(cd "$REPO_ROOT" && cd "$(git rev-parse --git-common-dir)/.." && pwd)"
fi

# Read worktree path from cycle-state.json (Builder runs inside it; post-ship
# the worktree may be removed — in that case verify against project-root HEAD).
WT=""
CS="$REPO_ROOT/.evolve/cycle-state.json"
if [ -f "$CS" ] && command -v jq >/dev/null 2>&1; then
    WT=$(jq -r '.active_worktree // empty' "$CS" 2>/dev/null)
fi

if [ -n "$WT" ] && [ -d "$WT" ]; then
    tracked=$(git -C "$WT" ls-files .evolve/inbox/ 2>/dev/null \
        | grep -E '^\.evolve/inbox/[^/]+\.json$' \
        | wc -l | tr -d ' ')
    src="worktree index ($WT)"
else
    # Post-ship: worktree gone, check project-root HEAD.
    tracked=$(git -C "$REPO_ROOT" ls-tree -r HEAD -- .evolve/inbox/ 2>/dev/null \
        | awk '{print $4}' \
        | grep -E '^\.evolve/inbox/[^/]+\.json$' \
        | wc -l | tr -d ' ')
    src="REPO_ROOT HEAD"
fi

if [ "${tracked:-0}" -gt 0 ]; then
    echo "RED: $src still has $tracked tracked top-level inbox json file(s)"
    exit 1
fi
echo "GREEN: $src .evolve/inbox/ has 0 tracked top-level json files"
exit 0
