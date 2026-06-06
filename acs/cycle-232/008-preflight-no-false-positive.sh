#!/usr/bin/env bash
# AC-ID:         cycle-232-t2-AC3
# Description:   unrelated untracked main-side files do NOT trip the pre-flight (false-positive guard)
# Evidence:      go/internal/phases/ship/gitops_collider_test.go (TDD RED contract, cycle 232)
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: scout-report.md Task 2 (topology-handles-and-ship-preflight) AC3

set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

ROOT="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"

# Anti-vacuous guard (see 006).
grep -q 'func TestShipFromWorktree_NoCollider(' "$ROOT/go/internal/phases/ship/gitops_collider_test.go" 2>/dev/null \
  || { echo "RED: TestShipFromWorktree_NoCollider missing — RED contract deleted?" >&2; exit 1; }

# Behavioral: ship with an unrelated untracked main-side file must still
# ff-merge cleanly (pre-existing GREEN at baseline; regression guard against
# an over-eager pre-flight).
assert_go_test_pass ./internal/phases/ship/ 'TestShipFromWorktree_NoCollider' || exit 1

echo "GREEN: pre-flight does not block unrelated untracked files" >&2
exit 0
