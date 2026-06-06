#!/usr/bin/env bash
# AC-ID:         cycle-231-001
# Description:   checkUntrackedColliders behavioral contract: single-collider detected with correct code+class+message, multiple colliders all named, clean tree returns nil, gitignored files are not colliders
# Evidence:      go/internal/phases/ship/collider_test.go:TestColliderPreflight (subtests: single-collider, multiple-colliders, clean-main-tree, gitignored-untracked) + go/internal/core/shiperror.go:CodeGitUntrackedCollider
# Author:        tester
# Created:       2026-06-06T08:30:00Z
# Acceptance-of: build-report.md Changes row: gitops.go — Call checkUntrackedColliders pre-merge + shiperror.go — Define CodeGitUntrackedCollider

set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"

# Behavioral: run TestColliderPreflight with -v and verify each key subtest
# ran (not just that the suite exited 0 — we need to be sure the collider
# detection subtests actually executed, not that the test binary was vacuous).
dir=$(acs_go_module_dir)
out=$(cd "$dir" && go test -race -count=1 -v -run 'TestColliderPreflight' ./internal/phases/ship/ 2>&1); rc=$?
if [ "$rc" -ne 0 ]; then
  echo "RED: TestColliderPreflight exited $rc" >&2
  echo "$out" | tail -10 >&2
  exit 1
fi

# Anti-vacuity: every behavioral subtest must have run (cycle-137 mode).
for subtest in \
  "wire value pinned" \
  "single collider detected" \
  "multiple colliders all named" \
  "clean main tree returns nil" \
  "gitignored untracked file is not a collider"; do
  # Normalise spaces — Go replaces spaces with underscores in subtest output.
  normalized=$(echo "$subtest" | tr ' ' '_')
  if ! echo "$out" | grep -qF -- "--- PASS: TestColliderPreflight/$normalized"; then
    echo "RED: subtest '$subtest' did not PASS — collider behavioral contract incomplete" >&2
    exit 1
  fi
done

# Structural: collider_test.go must be git-tracked (cycle-93 dual-check —
# untracked test files may be silently dropped at ship, vacuating the regression).
T="go/internal/phases/ship/collider_test.go"
git -C "$WORKTREE" ls-files --error-unmatch "$T" >/dev/null 2>&1 \
  || { echo "RED: $T is untracked — behavioral contract will be dropped at ship" >&2; exit 1; }

echo "GREEN: TestColliderPreflight all behavioral subtests PASS; collider_test.go is git-tracked" >&2
exit 0
