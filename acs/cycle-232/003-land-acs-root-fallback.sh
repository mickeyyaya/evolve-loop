#!/usr/bin/env bash
# AC-ID:         cycle-232-t1-AC3
# Description:   cherry-picked TestACSSuiteRootFallback exists AND passes (absent/empty/malformed cycle-state)
# Evidence:      go/cmd/evolve/cmd_acs_test.go (from 201f7cb)
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: scout-report.md Task 1 (land-audited-resolution-fix) AC3

set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

ROOT="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"

# Anti-vacuous guard (see 002): test must exist for -run to mean anything.
grep -q 'func TestACSSuiteRootFallback(' "$ROOT/go/cmd/evolve/cmd_acs_test.go" 2>/dev/null \
  || { echo "RED: TestACSSuiteRootFallback absent — cmd_acs_test.go not cherry-picked from 201f7cb" >&2; exit 1; }

# Behavioral: the fallback matrix passes on the landed tree.
assert_go_test_pass ./cmd/evolve/... 'TestACSSuiteRootFallback' || exit 1

echo "GREEN: TestACSSuiteRootFallback present and passing" >&2
exit 0
