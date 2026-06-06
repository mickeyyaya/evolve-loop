#!/usr/bin/env bash
# acs-predicate: config-check — recipe-row content is an inherent doc-presence
# check; no subprocess emits the recipe chains. Grep waiver applies.
# AC-ID:         cycle-12-004
# Description:   accounting-close, product-discovery, and ops-incident recipe
#                rows each carry their spec §4.1 three-phase chain verbatim.
# Evidence:      agents/evolve-router.md goal-type rows
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: scout-report.md — extend-goal-type-recipes AC-4
set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
cd "$WORKTREE" || { echo "RED: cannot cd to $WORKTREE" >&2; exit 1; }

ROUTER="agents/evolve-router.md"

check_row() {
  local goal="$1"; shift
  local row; row=$(grep -E "^\| *${goal} *\|" "$ROUTER" | head -1)
  [ -n "$row" ] || { echo "RED: $goal row missing" >&2; exit 1; }
  local phase
  for phase in "$@"; do
    if ! echo "$row" | grep -q "$phase"; then
      echo "RED: '$phase' not in $goal recipe: $row" >&2; exit 1
    fi
  done
}

check_row accounting-close   account-reconcile variance-analysis close-checklist
check_row product-discovery  opportunity-map prd-draft metric-tree
check_row ops-incident       incident-postmortem runbook-draft capacity-plan

echo "GREEN: accounting-close, product-discovery, ops-incident recipe chains correct" >&2
exit 0
