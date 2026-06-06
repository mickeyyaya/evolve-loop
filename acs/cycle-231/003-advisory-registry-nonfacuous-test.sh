#!/usr/bin/env bash
# AC-ID:         cycle-231-003
# Description:   TestRealRegistry_EvolveAdvisoryPinned is non-vacuous (matches >=1 test), passes, and exercises the real Load path; .evolve/phase-registry.json is git-tracked; MaxInsertions=6 passes TestLoad_RealRegistry
# Evidence:      go/internal/config/dynamic_routing_default_test.go:TestRealRegistry_EvolveAdvisoryPinned + go/internal/config/config_realregistry_test.go:TestLoad_RealRegistry (MaxInsertions=6) + .evolve/phase-registry.json git-tracking
# Author:        tester
# Created:       2026-06-06T08:30:00Z
# Acceptance-of: build-report.md Changes: .evolve/phase-registry.json created; config_realregistry_test.go MaxInsertions=6; Build Step #5

set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"

# Anti-vacuity check: run in -v mode and confirm the test function name appears
# in the PASS output (not just that the package exits 0 on an empty match).
# This is the cycle-231 TDD discovery — the cycle-227/008 predicate was
# vacuous because the test was never written until this cycle.
dir=$(acs_go_module_dir)
out=$(cd "$dir" && go test -race -count=1 -v -run 'TestRealRegistry_EvolveAdvisoryPinned' ./internal/config/ 2>&1); rc=$?
if [ "$rc" -ne 0 ]; then
  echo "RED: TestRealRegistry_EvolveAdvisoryPinned FAILED (exit $rc)" >&2
  echo "$out" | tail -10 >&2
  exit 1
fi
# Non-vacuity: the function must appear in the PASS line.
echo "$out" | grep -q 'PASS.*TestRealRegistry_EvolveAdvisoryPinned\|--- PASS: TestRealRegistry_EvolveAdvisoryPinned' \
  || { echo "RED: go test -v output does not show TestRealRegistry_EvolveAdvisoryPinned in PASS output — test is VACUOUS (matched zero functions)" >&2; exit 1; }
echo "GREEN: TestRealRegistry_EvolveAdvisoryPinned non-vacuously PASS" >&2

# MaxInsertions=6: TestLoad_RealRegistry now expects 6 (the registry was updated
# from 4 to 6 in wave-1). Run the test to confirm this assertion holds.
assert_go_test_pass ./internal/config/ 'TestLoad_RealRegistry' || exit 1

# Git-tracking: registry file must be tracked in the worktree (cycle-92 pattern
# — .evolve/ is gitignored by default; only the !whitelist entry saves it).
git -C "$WORKTREE" ls-files --error-unmatch ".evolve/phase-registry.json" >/dev/null 2>&1 \
  || { echo "RED: .evolve/phase-registry.json is untracked in worktree — cycle-92 drop risk" >&2; exit 1; }

# Whitelist entry: the .gitignore must carry the exception.
grep -q '!.evolve/phase-registry.json' "$WORKTREE/.gitignore" \
  || { echo "RED: .gitignore missing !.evolve/phase-registry.json whitelist entry" >&2; exit 1; }

echo "GREEN: advisory registry end-to-end verified (non-vacuous test + MaxInsertions=6 + tracking + whitelist)" >&2
exit 0
