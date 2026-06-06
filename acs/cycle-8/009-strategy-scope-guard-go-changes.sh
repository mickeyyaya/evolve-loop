#!/usr/bin/env bash
# AC-ID:         cycle-8-009
# Description:   Scope guard — the only Go file changed this cycle is usercatalog_research_test.go (Strategy wave is pure config; no production Go edits)
# Evidence:      git diff vs HEAD (worktree, pre-commit) — eval C5 mirrors this post-commit via HEAD~1
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: scout-report.md AC#5 — wave-business-strategy-tdd-and-phases
# NOTE: negative invariant — expected GREEN at RED baseline AND after build.

set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
cd "$WORKTREE" || { echo "RED: cannot cd to $WORKTREE" >&2; exit 1; }

# Behavioral: ask git (subprocess) for every .go path touched relative to HEAD,
# staged or not, plus untracked .go files. Filter out the one permitted file.
changed=$( { git diff HEAD --name-only; git ls-files --others --exclude-standard; } 2>/dev/null \
  | grep '\.go$' | grep -v 'usercatalog_research_test\.go' | sort -u || true)
if [ -n "$changed" ]; then
  echo "RED: unexpected Go file changes (scope creep): $changed" >&2
  exit 1
fi

echo "GREEN: only permitted Go file changed (usercatalog_research_test.go)" >&2
exit 0
