#!/usr/bin/env bash
# AC-ID: cycle-93-001-gitignore-profiles-md-exception
# AC-source: cycle-93/intent.md AC-1
# Behavioral predicate: .gitignore must contain the negation
#   `!.evolve/profiles/*.md` so that .evolve/profiles/AGENTS.md can be tracked.
#
# Root-cause anchor: cycle-92 failure — .gitignore:29 (`.evolve/profiles/*`)
# silently dropped .evolve/profiles/AGENTS.md at ship-time. The companion
# negation existed only for *.json. Adding *.md restores tracking.
#
# RED until Builder adds the exception line; GREEN once present.
# Bash 3.2 compatible. No GNU-only flags.
#
# Exit codes:
#   0 = GREEN (predicate satisfied)
#   1 = RED   (predicate violated)
set -uo pipefail

# Find repo root (worktree- or main-tree-agnostic).
REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)"
if [ -z "$REPO_ROOT" ]; then
  echo "RED: not inside a git work tree" >&2
  exit 1
fi
cd "$REPO_ROOT" || { echo "RED: cd failed" >&2; exit 1; }

if [ ! -f .gitignore ]; then
  echo "RED: .gitignore missing at repo root ($REPO_ROOT)" >&2
  exit 1
fi

# Match an explicit, leading-bang negation for *.md under .evolve/profiles/.
# Pattern is anchored loosely: allow whitespace-only leading variants but
# require the exact text. ERE used via grep -E.
if ! grep -Eq '^!\.evolve/profiles/\*\.md$' .gitignore; then
  echo "RED: .gitignore is missing line: !.evolve/profiles/*.md" >&2
  echo "current profiles-related lines:" >&2
  grep -n 'profiles' .gitignore >&2 || true
  exit 1
fi

# Cross-verify the gitignore actually no longer ignores AGENTS.md.
# check-ignore exits 0 when the path IS ignored, 1 when not, 128 on error.
# We expect a NON-ignored result for .evolve/profiles/AGENTS.md.
git check-ignore -q .evolve/profiles/AGENTS.md
ci_rc=$?
if [ "$ci_rc" -eq 0 ]; then
  echo "RED: git still ignores .evolve/profiles/AGENTS.md after negation" >&2
  git check-ignore -v .evolve/profiles/AGENTS.md >&2 || true
  exit 1
elif [ "$ci_rc" -eq 128 ]; then
  echo "RED: git check-ignore errored (rc=128)" >&2
  exit 1
fi

echo "GREEN: .gitignore contains !.evolve/profiles/*.md and AGENTS.md is not ignored"
exit 0
