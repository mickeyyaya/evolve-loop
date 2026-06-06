#!/usr/bin/env bash
# AC-ID:         cycle-230-002
# Description:   ValidateUserSpec enforces two-tier naming (multi-word kebab-case) for user phases; cycle-229 red anchor test GREEN and git-tracked
# Evidence:      go/internal/phasespec/validate.go (twoTierNameRE) + bug_repro_cycle229_test.go + two_tier_naming_cycle230_test.go
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: scout-report.md Task 2 (phase-naming-lint) — completes the TDD anchor placed in cycle 229

set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"

# Behavioral: the cycle-229 red anchor + cycle-230 acceptance/edge tests must
# pass (asserts on go test EXIT CODE via acs/lib/assert.sh — cycle-137 rule).
assert_go_test_pass ./internal/phasespec/... 'TestBugRepro_Cycle229_TwoTierNamingMissing|TestTwoTierNaming_MultiWordAccepted|TestTwoTierNaming_MalformedRejected' || exit 1

# No collateral damage: the rest of the phasespec package must stay green.
assert_go_test_pass ./internal/phasespec/... || exit 1

# Dual-check (cycle-93 rule): the red anchor must be tracked, not just on disk.
ANCHOR="go/internal/phasespec/bug_repro_cycle229_test.go"
[ -f "$WORKTREE/$ANCHOR" ] || { echo "RED: $ANCHOR missing on disk" >&2; exit 1; }
git -C "$WORKTREE" ls-files --error-unmatch "$ANCHOR" >/dev/null 2>&1 \
  || { echo "RED: $ANCHOR untracked — the cycle-229 TDD anchor must be committed with the fix" >&2; exit 1; }

echo "GREEN: two-tier naming gate enforced; anchor test green and tracked" >&2
exit 0
