#!/usr/bin/env bash
# AC-ID:         cycle-5-010
# Description:   3 accounting cases (account-reconcile, variance-analysis, close-checklist) PASS in TestResearchPhasesAreConfigOnly — accounting phase dirs present in the COMMITTED tree, not just untracked in the main tree
# Evidence:      go/internal/phasespec/usercatalog_research_test.go (cycle-5 cases) + .evolve/phases/{account-reconcile,variance-analysis,close-checklist}/
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: scout-report.md AC#10 — accounting-carry-forward-mirrors

set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
cd "$WORKTREE" || { echo "RED: cannot cd to $WORKTREE" >&2; exit 1; }
. "$WORKTREE/acs/lib/assert.sh"

# Anti-deletion guard (auxiliary): all 3 cases must still be in the test file,
# else `go test -run <regex>` matches nothing and exits 0.
for name in account-reconcile variance-analysis close-checklist; do
  grep -q "\"$name\": {" go/internal/phasespec/usercatalog_research_test.go \
    || { echo "RED: $name case removed from usercatalog_research_test.go" >&2; exit 1; }
done

# Behavioral: run the 3 accounting subtests; exit code is the authoritative signal.
assert_go_test_pass ./internal/phasespec/ 'TestResearchPhasesAreConfigOnly/(account-reconcile|variance-analysis|close-checklist)$' || exit 1

echo "GREEN: all 3 accounting catalog contracts hold" >&2
exit 0
