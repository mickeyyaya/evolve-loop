#!/usr/bin/env bash
# AC-ID: cycle-173-011-eval-files-persisted
# AC-source: TDD-Engineer SKILL Step 6b — "Every task with a predicate
#   disposition MUST produce a PERSISTENT regression eval at .evolve/evals/<slug>.md."
#   "Persistent" is load-bearing: the eval caps FUTURE cycles' audit scores, so it
#   must survive ship. The file-existence dual-check rule (cycle-93+) applies.
#
# DISCOVERY (cycle-173): `.evolve/*` is gitignored at .gitignore:34 with no
# `!.evolve/evals/` negation, so eval files satisfy the auditor's worktree `[ -f ]`
# check but are SILENTLY DROPPED at ship (cycle-92 class). A disk-only check would
# pass while the eval never persists. This predicate dual-checks disk + git-tracking
# so the gap is RED until Builder adds the negation and stages the files.
#
# Builder fix: add to .gitignore (after the existing .evolve negations):
#     !.evolve/evals/
#     !.evolve/evals/*.md
#   then `git add .evolve/evals/*.md`.
#
# Exit: 0 = GREEN, 1 = RED. Bash 3.2 compatible.
set -uo pipefail

ROOT="$(git rev-parse --show-toplevel 2>/dev/null)"
[ -n "$ROOT" ] || { echo "RED: not in a git work tree" >&2; exit 1; }
cd "$ROOT" || { echo "RED: cd failed" >&2; exit 1; }

EVALS="
.evolve/evals/transient-bridge-retry.md
.evolve/evals/backfill-ledger-and-docs.md
"

rc=0
for f in $EVALS; do
  if [ ! -f "$f" ]; then
    echo "RED: $f missing on disk (required regression eval)" >&2
    rc=1
    continue
  fi
  if ! git ls-files --error-unmatch "$f" >/dev/null 2>&1; then
    echo "RED: $f untracked — gitignored by .evolve/* and will be dropped at ship." >&2
    echo "     Builder: add '!.evolve/evals/' + '!.evolve/evals/*.md' to .gitignore, then git add." >&2
    rc=1
  fi
done
[ "$rc" -eq 0 ] || exit 1

echo "PASS: both regression eval files exist on disk AND are git-tracked (will persist past ship)"
exit 0
