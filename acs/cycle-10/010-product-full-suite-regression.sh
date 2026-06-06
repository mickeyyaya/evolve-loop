#!/usr/bin/env bash
# AC-ID:         cycle-10-010
# Description:   Negative/regression — full TestResearchPhasesAreConfigOnly (all 17 cases: 14 prior waves + 3 Product) exits 0; adding Product did not break Strategy/PM/Ops/Accounting/research cases
# Evidence:      go/internal/phasespec/usercatalog_research_test.go full run (eval C1 + regression axis)
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: scout-report.md AC#1 — wave-product-discovery-tdd-and-phases

set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
cd "$WORKTREE" || { echo "RED: cannot cd to $WORKTREE" >&2; exit 1; }
. "$WORKTREE/acs/lib/assert.sh"

# Anti-deletion guard (auxiliary): all 3 Product cases must still be in the
# table, else the "full suite" no longer covers the Product wave.
for phase in opportunity-map prd-draft metric-tree; do
  grep -q "\"$phase\": {" go/internal/phasespec/usercatalog_research_test.go \
    || { echo "RED: $phase case removed from usercatalog_research_test.go" >&2; exit 1; }
done

# Behavioral: the whole test (no subtest filter); exit code is authoritative.
assert_go_test_pass ./internal/phasespec/ 'TestResearchPhasesAreConfigOnly$' || exit 1

echo "GREEN: full catalog suite passes — Product insertion caused no regression" >&2
exit 0
