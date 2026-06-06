#!/usr/bin/env bash
# AC-ID:         cycle-232-t2-AC2
# Description:   collider refusal names EVERY collider (multi-path, nested dirs) so one pass fixes all
# Evidence:      go/internal/phases/ship/gitops_collider_test.go (TDD RED contract, cycle 232)
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: scout-report.md Task 2 (topology-handles-and-ship-preflight) AC2

set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

ROOT="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"

# Anti-vacuous guard (see 006).
grep -q 'func TestShipFromWorktree_ColliderError_IsActionable(' "$ROOT/go/internal/phases/ship/gitops_collider_test.go" 2>/dev/null \
  || { echo "RED: TestShipFromWorktree_ColliderError_IsActionable missing — RED contract deleted?" >&2; exit 1; }

# Behavioral: two colliders (one nested) must BOTH be named in the error.
assert_go_test_pass ./internal/phases/ship/ 'TestShipFromWorktree_ColliderError_IsActionable' || exit 1

echo "GREEN: collider error names all colliding paths" >&2
exit 0
