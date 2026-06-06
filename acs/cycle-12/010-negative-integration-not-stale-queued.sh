#!/usr/bin/env bash
# acs-predicate: config-check — wave-status table rows are an inherent
# doc-presence check. Grep waiver applies.
# AC-ID:         cycle-12-010
# Description:   Negative — the Integration row must NOT retain the stale
#                "⬜ queued (cycle 6)" state. Anti-no-op signal: a build that
#                appends tables but forgets the status flip stays RED here.
# Evidence:      docs/architecture/domain-phase-catalog.md Integration row
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: scout-report.md — extend-phase-core-values AC-5
set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
cd "$WORKTREE" || { echo "RED: cannot cd to $WORKTREE" >&2; exit 1; }

CATALOG="docs/architecture/domain-phase-catalog.md"
row=$(grep -E '^\| *Integration *\|' "$CATALOG" | head -1)
[ -n "$row" ] || { echo "RED: Integration row missing from wave-status table" >&2; exit 1; }

if echo "$row" | grep -q '⬜'; then
  echo "RED: Integration row still shows ⬜ queued: $row" >&2; exit 1
fi
if echo "$row" | grep -q 'cycle 6'; then
  echo "RED: Integration row still references stale 'cycle 6': $row" >&2; exit 1
fi

echo "GREEN: Integration row carries no stale queued/cycle-6 reference" >&2
exit 0
