#!/usr/bin/env bash
# AC-ID:         cycle-233-AC2
# Description:   ship pre-flight collider detection + kernel-resolved acs -root implemented and exercised
# Evidence:      gitops_collider_test.go (collider, actionable error, false-positive guard) + cmd_acs root autosolve/fallback
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: intent.md AC2 (ship pre-flight collider detection + kernel-resolved acs -root)

set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

ROOT="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"

# Anti-vacuous guards (deleted tests would make -run match nothing → exit 0).
grep -q 'func TestShipFromWorktree_ColliderPreflight(' "$ROOT/go/internal/phases/ship/gitops_collider_test.go" 2>/dev/null \
  || { echo "RED: TestShipFromWorktree_ColliderPreflight missing — collider RED contract deleted?" >&2; exit 1; }
grep -q 'func TestShipFromWorktree_NoCollider(' "$ROOT/go/internal/phases/ship/gitops_collider_test.go" 2>/dev/null \
  || { echo "RED: TestShipFromWorktree_NoCollider missing — false-positive guard deleted?" >&2; exit 1; }
grep -q 'resolveACSSuiteRoot' "$ROOT/go/cmd/evolve/cmd_acs.go" 2>/dev/null \
  || { echo "RED: resolveACSSuiteRoot not in cmd_acs.go — kernel-resolved -root not landed" >&2; exit 1; }

# Behavioral (load-bearing): positive (refusal pre-commit, paths named),
# negative (unrelated untracked files don't block), and root resolution.
assert_go_test_pass ./internal/phases/ship/ 'TestShipFromWorktree_ColliderPreflight|TestShipFromWorktree_ColliderError_IsActionable|TestShipFromWorktree_NoCollider' || exit 1
assert_go_test_pass ./cmd/evolve/ 'TestACSSuiteRootAutosolve|TestACSSuiteRootFallback' || exit 1

echo "GREEN: collider pre-flight + kernel-resolved -root behaviorally green" >&2
exit 0
