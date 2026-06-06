#!/usr/bin/env bash
# acs-predicate: config-check — archetype is a declarative phase.json field;
# the behavioral consequence (verdict vocabulary presence/absence) is covered
# by predicates 001–003 via the Go contract test. This predicate pins the
# declared field itself per spec §3. Grep waiver per tdd-engineer
# predicate-quality classification (Auditor reviews validity).
# AC-ID:         cycle-10-013
# Description:   Archetype correctness — opportunity-map=plan, prd-draft=plan, metric-tree=evaluate (spec §3 Wave Product table); plan phases must NOT be evaluate. Cycle-9 audit FAIL fix: this waiver header was missing on the cycle-9 equivalent (Level 0, blocking_count=1).
# Evidence:      .evolve/phases/{opportunity-map,prd-draft,metric-tree}/phase.json
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: scout-report.md AC#7 — wave-product-discovery-tdd-and-phases

set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
cd "$WORKTREE" || { echo "RED: cannot cd to $WORKTREE" >&2; exit 1; }

fail=0
check_archetype() {
  local phase="$1" want="$2"
  local f=".evolve/phases/$phase/phase.json"
  if [ ! -f "$f" ]; then
    echo "RED: $f missing" >&2; fail=1; return
  fi
  if ! grep -q "\"archetype\": \"$want\"" "$f"; then
    echo "RED: $phase archetype != $want" >&2; fail=1
  fi
}

check_archetype opportunity-map plan
check_archetype prd-draft plan
check_archetype metric-tree evaluate

# Negative axis: plan phases must NOT carry the evaluate archetype.
for phase in opportunity-map prd-draft; do
  if [ -f ".evolve/phases/$phase/phase.json" ] \
    && grep -q '"archetype": "evaluate"' ".evolve/phases/$phase/phase.json"; then
    echo "RED: $phase incorrectly declared evaluate (spec §3 says plan)" >&2; fail=1
  fi
done
[ "$fail" -eq 0 ] || exit 1

echo "GREEN: archetypes match spec §3 (plan/plan/evaluate)" >&2
exit 0
