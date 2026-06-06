#!/usr/bin/env bash
# AC-ID:         cycle-6-003
# Description:   TestResearchPhasesAreConfigOnly/dependency-map PASSES — PM evaluate phase in merged catalog with spec §3 contract (Dependencies/Critical Path/Blockers + verdict vocabulary)
# Evidence:      go/internal/phasespec/usercatalog_research_test.go (cycle-6 cases) + .evolve/phases/dependency-map/phase.json
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: scout-report.md AC#3 — wave-pm-tdd-and-phases

set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
cd "$WORKTREE" || { echo "RED: cannot cd to $WORKTREE" >&2; exit 1; }
. "$WORKTREE/acs/lib/assert.sh"

# Anti-deletion guard (auxiliary, not load-bearing): the case must still be in
# the test file, else `go test -run <regex>` matches nothing and exits 0.
grep -q '"dependency-map": {' go/internal/phasespec/usercatalog_research_test.go \
  || { echo "RED: dependency-map case removed from usercatalog_research_test.go" >&2; exit 1; }

# Behavioral: run the actual subtest; exit code is the authoritative signal.
assert_go_test_pass ./internal/phasespec/ 'TestResearchPhasesAreConfigOnly/dependency-map$' || exit 1

echo "GREEN: dependency-map catalog contract holds" >&2
exit 0
