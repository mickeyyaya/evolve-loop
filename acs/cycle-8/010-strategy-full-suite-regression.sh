#!/usr/bin/env bash
# AC-ID:         cycle-8-010
# Description:   Negative/regression — full TestResearchPhasesAreConfigOnly (all 14 cases: 11 prior waves + 3 Strategy) exits 0; adding Strategy did not break PM/Ops/Accounting/research cases
# Evidence:      go/internal/phasespec/usercatalog_research_test.go full run (eval C1 + regression axis)
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: scout-report.md AC#1 — wave-business-strategy-tdd-and-phases

set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
cd "$WORKTREE" || { echo "RED: cannot cd to $WORKTREE" >&2; exit 1; }
. "$WORKTREE/acs/lib/assert.sh"

# Anti-deletion guard (auxiliary): all 3 Strategy cases must still be in the
# table, else the "full suite" no longer covers the Strategy wave.
for phase in forces-analysis market-sizing okr-draft; do
  grep -q "\"$phase\": {" go/internal/phasespec/usercatalog_research_test.go \
    || { echo "RED: $phase case removed from usercatalog_research_test.go" >&2; exit 1; }
done

# Behavioral: the whole test (no subtest filter); exit code is authoritative.
assert_go_test_pass ./internal/phasespec/ 'TestResearchPhasesAreConfigOnly$' || exit 1

echo "GREEN: full catalog suite passes — Strategy insertion caused no regression" >&2
exit 0
