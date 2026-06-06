#!/usr/bin/env bash
# AC-ID:         cycle-232-t1-AC4
# Description:   cherry-picked TestBugRepro_Cycle229_TwoTierNamingMissing exists AND passes (two-tier naming lint)
# Evidence:      go/internal/phasespec/bug_repro_cycle229_test.go (from 201f7cb)
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: scout-report.md Task 1 (land-audited-resolution-fix) AC4

set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

ROOT="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"

# Anti-vacuous guard (see 002): test must exist for -run to mean anything.
grep -q 'func TestBugRepro_Cycle229_TwoTierNamingMissing(' "$ROOT/go/internal/phasespec/bug_repro_cycle229_test.go" 2>/dev/null \
  || { echo "RED: TestBugRepro_Cycle229_TwoTierNamingMissing absent — bug_repro_cycle229_test.go not cherry-picked" >&2; exit 1; }

# Behavioral: the cycle-229 bug-repro passes on the landed tree.
assert_go_test_pass ./internal/phasespec/... 'TestBugRepro_Cycle229_TwoTierNamingMissing' || exit 1

echo "GREEN: cycle-229 two-tier-naming bug repro present and passing" >&2
exit 0
