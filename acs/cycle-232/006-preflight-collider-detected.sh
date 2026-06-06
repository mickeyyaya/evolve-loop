#!/usr/bin/env bash
# AC-ID:         cycle-232-t2-AC1
# Description:   ship collider pre-flight refuses (precondition class, path named) BEFORE the worktree commit
# Evidence:      go/internal/phases/ship/gitops_collider_test.go (TDD RED contract, cycle 232)
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: scout-report.md Task 2 (topology-handles-and-ship-preflight) AC1

set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

ROOT="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"

# Anti-vacuous guard: the test must exist (deleting it would make -run match
# nothing and exit 0).
grep -q 'func TestShipFromWorktree_ColliderPreflight(' "$ROOT/go/internal/phases/ship/gitops_collider_test.go" 2>/dev/null \
  || { echo "RED: TestShipFromWorktree_ColliderPreflight missing — RED contract deleted?" >&2; exit 1; }

# Behavioral: real git repos, real ship Run(); asserts refusal happens
# pre-commit, class=precondition, collider path named, main-side copy intact.
assert_go_test_pass ./internal/phases/ship/ 'TestShipFromWorktree_ColliderPreflight' || exit 1

echo "GREEN: collider pre-flight refuses before commit with named path" >&2
exit 0
