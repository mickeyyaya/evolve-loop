#!/usr/bin/env bash
# AC-ID:         cycle-12-005
# Description:   Scope guard (extend-goal-type-recipes) — doc-only task; no
#                Go source may be changed or introduced this cycle. Behavioral:
#                interrogates git tree state (diff vs HEAD + untracked files),
#                not source text.
# Evidence:      git diff HEAD --name-only; git ls-files --others
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: scout-report.md — extend-goal-type-recipes AC-5
# NOTE: negative invariant — expected GREEN at RED baseline AND after build.
set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
cd "$WORKTREE" || { echo "RED: cannot cd to $WORKTREE" >&2; exit 1; }

go_changed=$( { git diff HEAD --name-only; git ls-files --others --exclude-standard; } | grep '\.go$' || true)
if [ -n "$go_changed" ]; then
  echo "RED: scope guard violated — Go files touched in a doc-only cycle:" >&2
  echo "$go_changed" >&2
  exit 1
fi

echo "GREEN: no Go files changed (scope guard holds)" >&2
exit 0
