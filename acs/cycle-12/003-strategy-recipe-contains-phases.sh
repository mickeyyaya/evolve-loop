#!/usr/bin/env bash
# acs-predicate: config-check — recipe-row content is an inherent doc-presence
# check; no subprocess emits the recipe chain. Grep waiver applies.
# AC-ID:         cycle-12-003
# Description:   business-strategy recipe row contains forces-analysis →
#                market-sizing → okr-draft (spec §4.1 chain, verbatim phase
#                names).
# Evidence:      agents/evolve-router.md business-strategy row
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: scout-report.md — extend-goal-type-recipes AC-3
set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
cd "$WORKTREE" || { echo "RED: cannot cd to $WORKTREE" >&2; exit 1; }

ROUTER="agents/evolve-router.md"
row=$(grep -E '^\| *business-strategy *\|' "$ROUTER" | head -1)
[ -n "$row" ] || { echo "RED: business-strategy row missing" >&2; exit 1; }

for phase in forces-analysis market-sizing okr-draft; do
  if ! echo "$row" | grep -q "$phase"; then
    echo "RED: '$phase' not in business-strategy recipe: $row" >&2; exit 1
  fi
done

echo "GREEN: business-strategy recipe chain correct" >&2
exit 0
