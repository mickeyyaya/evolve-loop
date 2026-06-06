#!/usr/bin/env bash
# AC-ID:         cycle-6-010
# Description:   Negative/regression — full TestResearchPhasesAreConfigOnly (all 10 cases: 7 prior waves + 3 PM) exits 0; adding PM did not break Ops/Accounting/research cases
# Evidence:      go/internal/phasespec/usercatalog_research_test.go full run (eval C6 negative axis)
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: scout-report.md AC#10 — wave-pm-tdd-and-phases

set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
cd "$WORKTREE" || { echo "RED: cannot cd to $WORKTREE" >&2; exit 1; }
. "$WORKTREE/acs/lib/assert.sh"

# Anti-deletion guard (auxiliary): all 3 PM cases must still be in the table,
# else the "full suite" no longer covers the PM wave.
for phase in risk-register scope-baseline dependency-map; do
  grep -q "\"$phase\": {" go/internal/phasespec/usercatalog_research_test.go \
    || { echo "RED: $phase case removed from usercatalog_research_test.go" >&2; exit 1; }
done

# Behavioral: the whole test (no subtest filter); exit code is authoritative.
assert_go_test_pass ./internal/phasespec/ 'TestResearchPhasesAreConfigOnly$' || exit 1

echo "GREEN: full catalog suite passes — PM insertion caused no regression" >&2
exit 0
