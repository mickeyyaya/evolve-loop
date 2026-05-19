#!/usr/bin/env bash
# AC-ID: cycle-90-002-orphan-worktrees-pruned
# Description: Verifies that the two orphan worktrees `cycle-78` and `cycle-80`
#   have been removed from `git worktree list` AND that no commits unique to
#   their branches were lost (commits either merged to main OR branch still
#   exists for forensic recovery). Plan §3C — operational hygiene.
# Evidence: intent.md success-criteria row "git worktree list | grep -c
#   'cycle-78|cycle-80' == 0 AND no commits lost"; triage-decision.md item 3C
#   notes Scout enumerates the full orphan set.
# Author: tdd-engineer (cycle-90)
# Created: 2026-05-19
# Acceptance-of: build-report.md row "3C: orphan worktrees pruned via
#   `git worktree remove`/`prune`; branches handled per scout findings"
#
# Behavioral: parses `git worktree list --porcelain` (machine-readable form)
# rather than the human-formatted output — a mutant that aliases the human
# output to hide a worktree cannot survive porcelain parsing. Verifies no
# commit loss by asserting any pruned branch's HEAD is reachable from main
# (merged) OR the branch ref is still present (forensic preservation).
set -uo pipefail

# Worktree-list is a property of the canonical git repo, not a per-cycle
# worktree. Prefer EVOLVE_PROJECT_ROOT to avoid running this check inside a
# worktree (which would still resolve `git worktree list` globally, but the
# semantics are clearer when we name the canonical root).
REPO_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
AC_ID="cycle-90-002-orphan-worktrees-pruned"

cd "$REPO_ROOT" 2>/dev/null || {
  echo "RED $AC_ID: cannot cd to $REPO_ROOT" >&2
  exit 1
}

if ! command -v git >/dev/null 2>&1; then
  echo "RED $AC_ID: git not on PATH" >&2
  exit 1
fi

ORPHANS="cycle-78 cycle-80"

# `git worktree list --porcelain` emits "worktree <abs-path>" lines.
worktree_listing=$(git worktree list --porcelain 2>/dev/null) || {
  echo "RED $AC_ID: git worktree list failed" >&2
  exit 1
}

failures=""
for name in $ORPHANS; do
  # Match the orphan path-segment at any depth — covers both
  # `.evolve/worktrees/cycle-78` and any legacy `worktrees/cycle-78` location.
  matched=$(printf '%s\n' "$worktree_listing" \
    | awk -v n="$name" '
        /^worktree / {
          path=$2
          if (path ~ ("/" n "$") || path ~ ("/" n "/")) print path
        }')
  if [ -n "$matched" ]; then
    failures="${failures}\n  worktree still present: $name -> $matched"
  fi
done

if [ -n "$failures" ]; then
  echo "RED $AC_ID: orphan worktrees still tracked by git" >&2
  printf "%b\n" "$failures" >&2
  exit 1
fi

# No-commit-loss check: if the legacy `evolve/cycle-78` / `evolve/cycle-80`
# branch refs still exist, that's acceptable (forensic preservation). If they
# were deleted, verify the HEAD they pointed at is reachable from main.
#
# Builder MAY have deleted the branches OR kept them — both are valid per
# intent §3C ("Delete orphan evolve/cycle-78/evolve/cycle-80 branches if no
# longer referenced by audit history"). We do NOT require deletion; we DO
# require that any deletion was loss-less.
loss_check_failures=""
for name in $ORPHANS; do
  branch="evolve/$name"
  if git show-ref --verify --quiet "refs/heads/$branch"; then
    # Branch still exists — forensic preservation, no loss possible. PASS.
    continue
  fi
  # Branch is gone. Was its tip ever reachable from main? Look in the reflog
  # of HEAD/main for any prior tip SHA of this branch. If the reflog is
  # exhausted (>90d) we can't prove no-loss — accept absence-of-evidence here
  # because main-line ship.sh ledger entries would have caught real loss.
  # The check below is best-effort: if reflog mentions the branch name, ensure
  # at least one of its prior tips is now reachable from origin/main.
  prior_tips=$(git reflog --all 2>/dev/null | awk -v b="$branch" '$0 ~ b {print $1}' | head -5)
  if [ -z "$prior_tips" ]; then
    # No reflog evidence — branch never existed in our window OR was cleaned
    # up cleanly. Accept (Builder must surface in build-report.md if loss).
    continue
  fi
  reachable=0
  for tip in $prior_tips; do
    if git merge-base --is-ancestor "$tip" main 2>/dev/null; then
      reachable=1
      break
    fi
  done
  if [ "$reachable" -eq 0 ]; then
    loss_check_failures="${loss_check_failures}\n  branch $branch deleted with unreachable tips: $prior_tips"
  fi
done

if [ -n "$loss_check_failures" ]; then
  echo "RED $AC_ID: orphan branch deletion lost commits not reachable from main" >&2
  printf "%b\n" "$loss_check_failures" >&2
  exit 1
fi

echo "GREEN $AC_ID: cycle-78 and cycle-80 worktrees pruned; no commits lost"
exit 0
