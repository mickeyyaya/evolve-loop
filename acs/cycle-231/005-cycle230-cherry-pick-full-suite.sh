#!/usr/bin/env bash
# AC-ID:         cycle-231-005
# Description:   Cherry-pick cycle-230 content integrated via observable effects: two-tier naming + ACS-root-autosolve + ledger-source tests pass; auditor persona <=300 lines with git-tracked reference file; full go test green
# Evidence:      go/internal/phasespec/(two_tier_naming_cycle230_test.go + bug_repro_cycle229_test.go) + go/cmd/evolve/cmd_acs_test.go + go/internal/core/ledger_source_test.go + agents/evolve-auditor.md (line count) + agents/evolve-auditor-reference.md (tracked)
# Author:        tester
# Created:       2026-06-06T08:30:00Z
# Acceptance-of: build-report.md Build Step #3 cherry-pick-cycle-230 (AC1.3–AC1.5) + Build Step #5 auditor-doc-trim

set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"

# Two-tier naming tests: cycle-230 cherry-pick brings TestTwoTierNaming and
# TestBugRepro_Cycle229_TwoTierNamingMissing. Verify they exist and pass.
assert_go_test_pass ./internal/phasespec/... 'TestTwoTierNaming|TestBugRepro_Cycle229_TwoTierNamingMissing' || exit 1

# ACS suite root autosolve tests: cmd_acs_test.go from cycle-230.
assert_go_test_pass ./cmd/evolve/... 'TestACSSuiteRoot' || exit 1

# Ledger source tests: ledger_source_test.go from cycle-230.
assert_go_test_pass ./internal/core/... 'TestLedgerEntrySource' || exit 1

# Auditor persona line count: the trim must hold at <=300 lines.
AUDITOR="$WORKTREE/agents/evolve-auditor.md"
[ -f "$AUDITOR" ] || { echo "RED: $AUDITOR missing" >&2; exit 1; }
lines=$(wc -l < "$AUDITOR" | tr -d ' ')
if [ "$lines" -gt 300 ]; then
  echo "RED: agents/evolve-auditor.md is $lines lines (> 300) — auditor-doc-trim not merged" >&2
  exit 1
fi
echo "GREEN: evolve-auditor.md is $lines lines (<= 300)" >&2

# Reference file: the offloaded content must exist AND be in the index (cycle-93
# dual-check — cycle-92 dropped untracked .md files silently at ship).
REF="agents/evolve-auditor-reference.md"
[ -f "$WORKTREE/$REF" ] || { echo "RED: $REF missing — offloaded content not present" >&2; exit 1; }
git -C "$WORKTREE" ls-files -- "$REF" | grep -q . \
  || { echo "RED: $REF not in index — cycle-92 drop risk at ship" >&2; exit 1; }
echo "GREEN: $REF exists and is in the git index" >&2

# Full go test: all packages green — covers the cherry-picked sub-tasks
# (naming-lint, ACS-root-autosolve, ledger-source) and the new collider code,
# detecting any collateral damage.
assert_go_test_pass ./... || exit 1

echo "GREEN: cherry-pick cycle-230 integration verified via test outcomes: naming/ACS-root/ledger/collider + auditor trimmed + full suite green" >&2
exit 0
