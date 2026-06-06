#!/usr/bin/env bash
# acs-predicate: config-check — core-values table rows are an inherent
# doc-presence check; no subprocess emits the catalog table. Grep waiver
# per tdd-engineer predicate-quality classification.
# AC-ID:         cycle-12-006
# Description:   All 15 domain phase rows (PM/Strategy/Accounting/Product/Ops
#                waves, spec §3) present in the Phase Catalog — Core Values
#                table of agents/evolve-router.md, each with a non-empty
#                core-value cell.
# Evidence:      agents/evolve-router.md Core Values table
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: scout-report.md — extend-phase-core-values AC-1
set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
cd "$WORKTREE" || { echo "RED: cannot cd to $WORKTREE" >&2; exit 1; }

ROUTER="agents/evolve-router.md"
[ -f "$ROUTER" ] || { echo "RED: $ROUTER missing" >&2; exit 1; }

for phase in \
  risk-register scope-baseline dependency-map \
  forces-analysis market-sizing okr-draft \
  account-reconcile variance-analysis close-checklist \
  opportunity-map prd-draft metric-tree \
  incident-postmortem runbook-draft capacity-plan
do
  row=$(grep -E "^\| *\`?${phase}\`? *\|" "$ROUTER" | head -1)
  if [ -z "$row" ]; then
    echo "RED: core-values row for '$phase' not found in $ROUTER" >&2; exit 1
  fi
  # Reject placeholder rows: the core-value cell (2nd column) must be non-empty.
  value=$(echo "$row" | awk -F'|' '{ gsub(/^ +| +$/, "", $3); print $3 }')
  if [ -z "$value" ]; then
    echo "RED: core-values row for '$phase' has an empty value cell: $row" >&2; exit 1
  fi
done

echo "GREEN: all 15 domain phase rows present with non-empty core values" >&2
exit 0
