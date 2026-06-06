#!/usr/bin/env bash
# AC-ID:         cycle-231-002
# Description:   checkUntrackedColliders is called BEFORE the --ff-only merge in gitops.go; CodeGitUntrackedCollider wire value is "GIT_UNTRACKED_COLLIDER" (distinct from GIT_FF_MERGE_DIVERGED — the cycle-230 incident that conflated the two)
# Evidence:      go/internal/phases/ship/gitops.go:268 (preflight call) < gitops.go:273 (ff-merge call) + go/internal/core/shiperror.go:CodeGitUntrackedCollider
# Author:        tester
# Created:       2026-06-06T08:30:00Z
# Acceptance-of: build-report.md Changes row: gitops.go — Call checkUntrackedColliders pre-merge; shiperror.go — Define CodeGitUntrackedCollider; Build Step #4

set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"

GITOPS="$WORKTREE/go/internal/phases/ship/gitops.go"
[ -f "$GITOPS" ] || { echo "RED: gitops.go not found at $GITOPS" >&2; exit 1; }

# Placement: checkUntrackedColliders must appear BEFORE the --ff-only argument.
# awk emits the line number of each; we compare them.
preflight_line=$(grep -n 'checkUntrackedColliders' "$GITOPS" | grep -v '^.*func ' | head -1 | cut -d: -f1)
ffmerge_line=$(grep -n -- '--ff-only' "$GITOPS" | head -1 | cut -d: -f1)

if [ -z "$preflight_line" ]; then
  echo "RED: checkUntrackedColliders call not found in $GITOPS" >&2; exit 1
fi
if [ -z "$ffmerge_line" ]; then
  echo "RED: --ff-only not found in $GITOPS (ff-merge call missing?)" >&2; exit 1
fi
if [ "$preflight_line" -ge "$ffmerge_line" ]; then
  echo "RED: checkUntrackedColliders (line $preflight_line) is NOT before --ff-only (line $ffmerge_line) — preflight fires AFTER the merge it is supposed to guard" >&2
  exit 1
fi
echo "GREEN: checkUntrackedColliders at line $preflight_line < --ff-only at line $ffmerge_line (correct placement)" >&2

# Wire value: CodeGitUntrackedCollider must be "GIT_UNTRACKED_COLLIDER", not the
# GIT_FF_MERGE_DIVERGED value that the cycle-230 ship produced for this failure
# mode (making the failure undistinguishable in the ledger/debugger).
# The wire_value_pinned subtest from TestColliderPreflight pins exactly this, so
# asserting it passes here is the behavioral verification.
assert_go_test_pass ./internal/phases/ship/ 'TestColliderPreflight/wire_value_pinned' || exit 1

# Also check the constant text directly as a complementary structural check.
SHIPERR="$WORKTREE/go/internal/core/shiperror.go"
grep -q 'CodeGitUntrackedCollider.*ShipErrorCode.*=.*"GIT_UNTRACKED_COLLIDER"' "$SHIPERR" \
  || { echo "RED: CodeGitUntrackedCollider wire value not 'GIT_UNTRACKED_COLLIDER' in shiperror.go" >&2; exit 1; }

echo "GREEN: placement correct + wire value GIT_UNTRACKED_COLLIDER confirmed" >&2
exit 0
