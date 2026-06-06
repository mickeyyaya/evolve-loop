#!/usr/bin/env bash
# acs-predicate: config-check — archetype is a declarative phase.json field;
# the behavioral consequence (verdict vocabulary presence/absence) is covered
# by predicates 001–003 via the Go contract test. This predicate pins the
# declared field itself per spec §3. Grep waiver per tdd-engineer
# predicate-quality classification (Auditor reviews validity).
# AC-ID:         cycle-8-013
# Description:   Archetype correctness — forces-analysis=evaluate, market-sizing=evaluate, okr-draft=plan (spec §3 Wave Strategy table); okr-draft must NOT be evaluate
# Evidence:      .evolve/phases/{forces-analysis,market-sizing,okr-draft}/phase.json
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: scout-report.md AC#7 — wave-business-strategy-tdd-and-phases

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

check_archetype forces-analysis evaluate
check_archetype market-sizing evaluate
check_archetype okr-draft plan

# Negative axis: okr-draft must NOT carry the evaluate archetype.
if [ -f ".evolve/phases/okr-draft/phase.json" ] \
  && grep -q '"archetype": "evaluate"' ".evolve/phases/okr-draft/phase.json"; then
  echo "RED: okr-draft incorrectly declared evaluate (spec says plan)" >&2; fail=1
fi
[ "$fail" -eq 0 ] || exit 1

echo "GREEN: archetypes match spec §3 (evaluate/evaluate/plan)" >&2
exit 0
