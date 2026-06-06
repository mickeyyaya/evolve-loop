#!/usr/bin/env bash
# AC-ID:         cycle-233-AC1
# Description:   201f7cb phase-identity resolution content live on the cycle branch (behavioral, no dropped hunks)
# Evidence:      cherry-picked cycle-230/232 test contract (cmd_acs_test.go, ledger_source_test.go, two-tier naming tests)
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: intent.md AC1 (201f7cb content present, no dropped hunks vs rescue/cycle-232-audited)

set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

ROOT="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"

# Anti-vacuous guards: the RED-contract tests must exist — a deleted test file
# makes -run match nothing and `go test` exit 0 (the false-GREEN footgun).
grep -q 'func TestACSSuiteRootAutosolve(' "$ROOT/go/cmd/evolve/cmd_acs_test.go" 2>/dev/null \
  || { echo "RED: TestACSSuiteRootAutosolve missing — resolution-fix test contract absent" >&2; exit 1; }
grep -q 'func TestLedgerEntrySource_FieldPresent(' "$ROOT/go/internal/core/ledger_source_test.go" 2>/dev/null \
  || { echo "RED: TestLedgerEntrySource_FieldPresent missing — ledger-source test contract absent" >&2; exit 1; }
grep -q 'func TestBugRepro_Cycle229_TwoTierNamingMissing(' "$ROOT/go/internal/phasespec/bug_repro_cycle229_test.go" 2>/dev/null \
  || { echo "RED: cycle-229 two-tier naming bug-repro missing" >&2; exit 1; }

# Behavioral (load-bearing): exit-code assertions per cycle-137. These exercise
# resolveACSSuiteRoot, LedgerEntry.Source round-trip, and ValidateUserSpec lint.
assert_go_test_pass ./cmd/evolve/ 'TestACSSuiteRoot' || exit 1
assert_go_test_pass ./internal/core/ 'TestLedgerEntrySource' || exit 1
assert_go_test_pass ./internal/phasespec/ 'TestBugRepro_Cycle229|TestTwoTierNaming' || exit 1

echo "GREEN: 201f7cb resolution content present and behaviorally green" >&2
exit 0
