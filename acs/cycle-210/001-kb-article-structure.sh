#!/usr/bin/env bash
# ACS cycle-210 / Task-1 AC1 — KB article exists, git-TRACKED, and has >=5
# top-level (##) sections.
#
# DUAL CHECK (cycle-92/209 lesson): file-existence predicates MUST verify git
# tracking, not just disk presence. Cycle-209 FAILED precisely because a
# deliverable existed in the worktree on disk but was untracked, so it was
# silently dropped at ship. `[ -f ]` alone passes for an untracked file;
# `git ls-files --error-unmatch` is the load-bearing guard.
#
# RED at baseline (file absent / untracked); GREEN once Builder writes AND
# stages the KB article with >=5 sections.
set -uo pipefail
TOP=$(git rev-parse --show-toplevel) || { echo "RED: not a git repo" >&2; exit 1; }
cd "$TOP" || { echo "RED: cannot cd to repo root" >&2; exit 1; }

FILE="knowledge-base/research/missing-development-phases-2026-06.md"

# Check 1: disk presence
[ -f "$FILE" ] || { echo "RED: $FILE missing on disk" >&2; exit 1; }

# Check 2: git tracking — catches untracked/gitignored worktree files
git ls-files --error-unmatch "$FILE" >/dev/null 2>&1 \
  || { echo "RED: $FILE untracked — would be dropped at ship (cycle-209 mode)" >&2; exit 1; }

# Check 3: structural — >=5 top-level sections
SECTIONS=$(grep -c "^## " "$FILE" || true)
if [ "$SECTIONS" -lt 5 ]; then
  echo "RED: $FILE has only $SECTIONS top-level (##) sections, expected >= 5" >&2
  exit 1
fi

echo "GREEN: $FILE tracked with $SECTIONS top-level sections" >&2
exit 0
