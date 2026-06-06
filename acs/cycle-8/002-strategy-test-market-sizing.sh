#!/usr/bin/env bash
# AC-ID:         cycle-8-002
# Description:   TestResearchPhasesAreConfigOnly/market-sizing PASSES — Strategy evaluate phase in merged catalog with spec §3 contract (TAM/SAM/SOM/Methodology and Assumptions + verdict vocabulary)
# Evidence:      go/internal/phasespec/usercatalog_research_test.go (cycle-8 cases) + .evolve/phases/market-sizing/phase.json
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: scout-report.md AC#1 — wave-business-strategy-tdd-and-phases

set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
cd "$WORKTREE" || { echo "RED: cannot cd to $WORKTREE" >&2; exit 1; }
. "$WORKTREE/acs/lib/assert.sh"

# Anti-deletion guard (auxiliary, not load-bearing): the case must still be in
# the test file, else `go test -run <regex>` matches nothing and exits 0.
grep -q '"market-sizing": {' go/internal/phasespec/usercatalog_research_test.go \
  || { echo "RED: market-sizing case removed from usercatalog_research_test.go" >&2; exit 1; }

# Behavioral: run the actual subtest; exit code is the authoritative signal.
assert_go_test_pass ./internal/phasespec/ 'TestResearchPhasesAreConfigOnly/market-sizing$' || exit 1

echo "GREEN: market-sizing catalog contract holds" >&2
exit 0
