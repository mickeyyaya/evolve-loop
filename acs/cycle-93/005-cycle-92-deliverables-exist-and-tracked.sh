#!/usr/bin/env bash
# AC-ID: cycle-93-005-cycle-92-deliverables-exist-and-tracked
# AC-source: cycle-93/intent.md AC-5
# Behavioral predicate: the five cycle-92 deliverables that were
# silently dropped at ship must (a) exist on disk, (b) be tracked by
# git, and (c) have non-trivial content (>=30 non-blank lines for the
# AGENTS.md docs; >=20 non-blank lines for CODEBASE-MAP).
#
# This is the regression replacement for the lost cycle-92 predicates
# 001 (subdir-agents-md-exists) and 005 (profiles-agents-md-schema).
# Dual-check pattern: combine `[ -f ]` with `git ls-files
# --error-unmatch` so a worktree-only file fails.
#
# Bash 3.2 compatible. Iterates a tuple stream via printf | while read.
#
# Exit codes:
#   0 = GREEN (all five deliverables present, tracked, non-trivial)
#   1 = RED   (at least one fails)
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)"
if [ -z "$REPO_ROOT" ]; then
  echo "RED: not inside a git work tree" >&2
  exit 1
fi
cd "$REPO_ROOT" || { echo "RED: cd failed" >&2; exit 1; }

# Stream of "path:min_nonblank_lines" tuples.
# AGENTS.md docs require 30 lines (per cycle-92 plan); CODEBASE-MAP
# requires 20 lines (it's a directory map, naturally shorter).
TUPLES="\
agents/AGENTS.md:30
scripts/AGENTS.md:30
acs/AGENTS.md:30
.evolve/profiles/AGENTS.md:30
docs/CODEBASE-MAP.md:20"

fail_count=0
fail_summary=""

# Process substitution avoided for bash 3.2 sub-shell safety; use a
# pipe + while-read pattern. fail_count is tallied inside the loop and
# echoed back to a tmpfile because pipe subshells lose variable
# updates in bash 3.2.
TMP_FAIL="$(mktemp -t cycle93-005.XXXXXX)" || { echo "RED: mktemp failed" >&2; exit 1; }
: > "$TMP_FAIL"

printf '%s\n' "$TUPLES" | while IFS=: read -r path min_lines; do
  [ -z "$path" ] && continue

  if [ ! -f "$path" ]; then
    printf 'MISSING:%s\n' "$path" >> "$TMP_FAIL"
    continue
  fi

  if ! git ls-files --error-unmatch "$path" >/dev/null 2>&1; then
    printf 'UNTRACKED:%s\n' "$path" >> "$TMP_FAIL"
    continue
  fi

  # Count non-blank lines portably (BSD/GNU grep both honor -c -v -E).
  nonblank=$(grep -cvE '^[[:space:]]*$' "$path" 2>/dev/null || echo 0)
  if [ "$nonblank" -lt "$min_lines" ]; then
    printf 'THIN:%s(%s<%s)\n' "$path" "$nonblank" "$min_lines" >> "$TMP_FAIL"
    continue
  fi
done

if [ -s "$TMP_FAIL" ]; then
  while IFS= read -r line; do
    [ -z "$line" ] && continue
    fail_count=$(( fail_count + 1 ))
    fail_summary="$fail_summary  - $line"$'\n'
  done < "$TMP_FAIL"
  rm -f "$TMP_FAIL"

  printf 'RED: %s cycle-92 deliverable(s) failed:\n' "$fail_count" >&2
  printf '%s' "$fail_summary" >&2
  exit 1
fi
rm -f "$TMP_FAIL"

echo "GREEN: all 5 cycle-92 deliverables exist, are git-tracked, and meet line-count minima"
exit 0
