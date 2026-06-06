#!/usr/bin/env bash
# AC-ID:         cycle-232-t1-AC2
# Description:   cherry-picked TestACSSuiteRootAutosolve exists AND passes
# Evidence:      go/cmd/evolve/cmd_acs_test.go (from 201f7cb)
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: scout-report.md Task 1 (land-audited-resolution-fix) AC2

set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

ROOT="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"

# Anti-vacuous guard: `go test -run X` exits 0 when X matches NOTHING, so the
# test function must exist before the run result means anything.
grep -q 'func TestACSSuiteRootAutosolve(' "$ROOT/go/cmd/evolve/cmd_acs_test.go" 2>/dev/null \
  || { echo "RED: TestACSSuiteRootAutosolve absent — cmd_acs_test.go not cherry-picked from 201f7cb" >&2; exit 1; }

# Behavioral: the test passes on the landed tree.
assert_go_test_pass ./cmd/evolve/... 'TestACSSuiteRootAutosolve' || exit 1

echo "GREEN: TestACSSuiteRootAutosolve present and passing" >&2
exit 0
