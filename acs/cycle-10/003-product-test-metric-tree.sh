#!/usr/bin/env bash
# AC-ID:         cycle-10-003
# Description:   TestResearchPhasesAreConfigOnly/metric-tree PASSES — Product evaluate phase in merged catalog with spec §3 contract (North Star Metric/Input Metrics/Guardrail Metrics + verdict vocabulary). Covers AC#6 too: the Go case asserts c.Verdicts non-empty (evaluate + verdict_on_pass derivation).
# Evidence:      go/internal/phasespec/usercatalog_research_test.go (cycle-10 cases) + .evolve/phases/metric-tree/phase.json
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: scout-report.md AC#1 + AC#6 — wave-product-discovery-tdd-and-phases

set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
cd "$WORKTREE" || { echo "RED: cannot cd to $WORKTREE" >&2; exit 1; }
. "$WORKTREE/acs/lib/assert.sh"

# Anti-deletion guard (auxiliary, not load-bearing): the case must still be in
# the test file, else `go test -run <regex>` matches nothing and exits 0.
grep -q '"metric-tree": {' go/internal/phasespec/usercatalog_research_test.go \
  || { echo "RED: metric-tree case removed from usercatalog_research_test.go" >&2; exit 1; }

# Anti-dilution guard (auxiliary): the case must keep hasVerdict: true — the
# verdict assertion is what makes this the AC#6 oracle. (-A4: the want struct
# literal is 4 lines; awk /{/,/}/ ranges break on the sections line's `}`.)
grep -A4 '"metric-tree": {' go/internal/phasespec/usercatalog_research_test.go \
  | grep -q 'hasVerdict: true' \
  || { echo "RED: metric-tree case no longer asserts hasVerdict: true" >&2; exit 1; }

# Behavioral: run the actual subtest; exit code is the authoritative signal.
assert_go_test_pass ./internal/phasespec/ 'TestResearchPhasesAreConfigOnly/metric-tree$' || exit 1

echo "GREEN: metric-tree catalog contract holds (incl. verdict vocabulary)" >&2
exit 0
