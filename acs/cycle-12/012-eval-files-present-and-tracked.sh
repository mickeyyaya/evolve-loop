#!/usr/bin/env bash
# AC-ID:         cycle-12-012
# Description:   Both cycle-12 eval files exist AND are git-tracked
#                (file-existence dual-check rule, cycle-93+: [ -f ] alone
#                passes on a gitignored file that ship silently drops —
#                cycle-92 defect mode). Guards the cycle-131 lesson:
#                missing .evolve/evals/<slug>.md = automatic CRITICAL FAIL.
# Evidence:      test -f + git ls-files --error-unmatch on both eval files
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: TDD-engineer Step 6b contract (supplementary; both tasks)
# NOTE: RED until Builder stages the eval files (`git add`) — untracked new
#       files are the correct RED signal at pre-implementation baseline.
set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
cd "$WORKTREE" || { echo "RED: cannot cd to $WORKTREE" >&2; exit 1; }

for path in .evolve/evals/extend-goal-type-recipes.md .evolve/evals/extend-phase-core-values.md; do
  # Check 1: disk presence
  [ -f "$path" ] || { echo "RED: $path missing on disk" >&2; exit 1; }
  # Check 2: git tracking — catches gitignored/unstaged worktree files
  git ls-files --error-unmatch "$path" >/dev/null 2>&1 \
    || { echo "RED: $path untracked — stage it (git add) or it is silently dropped at ship" >&2; exit 1; }
done

echo "GREEN: both cycle-12 eval files present and git-tracked" >&2
exit 0
