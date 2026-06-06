#!/usr/bin/env bash
# AC-ID:         cycle-5-002
# Description:   TestResearchPhasesAreConfigOnly/runbook-draft PASSES — Ops control phase in merged catalog with spec §3 contract (Trigger/Diagnosis/Resolution Steps/Escalation, NO verdict vocabulary)
# Evidence:      go/internal/phasespec/usercatalog_research_test.go (cycle-5 cases) + .evolve/phases/runbook-draft/phase.json
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: scout-report.md AC#2 — wave-ops-tdd-and-phases

set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
cd "$WORKTREE" || { echo "RED: cannot cd to $WORKTREE" >&2; exit 1; }
. "$WORKTREE/acs/lib/assert.sh"

# Anti-deletion guard (auxiliary): empty -run match would exit 0.
grep -q '"runbook-draft": {' go/internal/phasespec/usercatalog_research_test.go \
  || { echo "RED: runbook-draft case removed from usercatalog_research_test.go" >&2; exit 1; }

# Behavioral: run the actual subtest; exit code is the authoritative signal.
assert_go_test_pass ./internal/phasespec/ 'TestResearchPhasesAreConfigOnly/runbook-draft$' || exit 1

echo "GREEN: runbook-draft catalog contract holds" >&2
exit 0
