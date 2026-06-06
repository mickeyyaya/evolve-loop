#!/usr/bin/env bash
# acs-predicate: config-check — table-row counting is an inherent doc-presence
# check; no subprocess emits the table. Grep waiver applies.
# AC-ID:         cycle-12-008
# Description:   Phase Catalog — Core Values table totals ≥29 phase rows
#                (14 original software-dev + 15 domain). Counts backticked
#                phase rows only, matching the table's row shape.
# Evidence:      agents/evolve-router.md Core Values table row count
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: scout-report.md — extend-phase-core-values AC-3
set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
cd "$WORKTREE" || { echo "RED: cannot cd to $WORKTREE" >&2; exit 1; }

ROUTER="agents/evolve-router.md"
[ -f "$ROUTER" ] || { echo "RED: $ROUTER missing" >&2; exit 1; }

count=$(grep -cE '^\| *`[a-z][a-z-]+` *\|' "$ROUTER" || true)
if [ "${count:-0}" -lt 29 ]; then
  echo "RED: Core Values table has only ${count:-0} phase rows; expected ≥29 (14 original + 15 domain)" >&2
  exit 1
fi

echo "GREEN: Core Values table has $count phase rows (≥29)" >&2
exit 0
