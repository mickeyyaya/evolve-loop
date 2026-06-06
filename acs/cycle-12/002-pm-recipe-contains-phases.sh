#!/usr/bin/env bash
# acs-predicate: config-check — recipe-row content is an inherent doc-presence
# check; no subprocess emits the recipe chain. Grep waiver applies.
# AC-ID:         cycle-12-002
# Description:   project-management recipe row contains risk-register →
#                scope-baseline → dependency-map (spec §4.1 chain, verbatim
#                phase names).
# Evidence:      agents/evolve-router.md project-management row
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: scout-report.md — extend-goal-type-recipes AC-2
set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
cd "$WORKTREE" || { echo "RED: cannot cd to $WORKTREE" >&2; exit 1; }

ROUTER="agents/evolve-router.md"
row=$(grep -E '^\| *project-management *\|' "$ROUTER" | head -1)
[ -n "$row" ] || { echo "RED: project-management row missing" >&2; exit 1; }

for phase in risk-register scope-baseline dependency-map; do
  if ! echo "$row" | grep -q "$phase"; then
    echo "RED: '$phase' not in project-management recipe: $row" >&2; exit 1
  fi
done

echo "GREEN: project-management recipe chain correct" >&2
exit 0
