#!/usr/bin/env bash
# AC-ID: cycle-103-006-phase-gate-precondition-recognizes-build-planner
# AC-source: scout-report.md AC-5 (lines 324, 363-366), edit spec lines 192-208
# Behavioral predicate:
#   scripts/guards/phase-gate-precondition.sh must include "build-planner"
#   in its case-statement alternation (not just inside a comment).
#
#   Anchored pattern: `|build-planner|` or `|build-planner)` -- the leading
#   pipe ensures membership in a `case` alternation. A bare comment mention
#   ("# add build-planner here") would not match this anchor.
#
# Mutation spec (cycle-103-006-MUT):
#   Mutant: "build-planner" only in a comment       -> must FAIL (no |build-planner anchor).
#   Mutant: only "build-planner-worker-*" present   -> must FAIL (both required).
#   Mutant: build-planner absent                    -> must FAIL.
#
# Bash 3.2 compatible.
#
# Exit codes:
#   0 = GREEN
#   1 = RED
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)"
if [ -z "$REPO_ROOT" ]; then
  echo "RED: not inside a git work tree" >&2
  exit 1
fi
cd "$REPO_ROOT" || { echo "RED: cd failed" >&2; exit 1; }

TARGET="scripts/guards/phase-gate-precondition.sh"

if [ ! -f "$TARGET" ]; then
  echo "RED: $TARGET does not exist" >&2
  exit 1
fi

# Anchor-based: |build-planner| or |build-planner) -- alternation membership.
# This deliberately rejects "# build-planner" comments (no leading pipe).
if ! grep -Eq '\|build-planner[\|\)]' "$TARGET"; then
  echo "RED: $TARGET case-statement does not include 'build-planner' as an alternation member" >&2
  echo "Expected: a line matching the regex \\|build-planner[\\|\\)]" >&2
  grep -n 'build-planner' "$TARGET" >&2 || echo "(no occurrences of 'build-planner' at all)" >&2
  exit 1
fi

# Additional sanity: the worker-pattern variant should ALSO be present
# (scout-report.md edit spec lines 202-208 requires both lines updated).
if ! grep -Eq 'build-planner-worker-\*' "$TARGET"; then
  echo "RED: $TARGET worker-pattern alternation does not include 'build-planner-worker-*'" >&2
  grep -n 'worker-\*' "$TARGET" >&2 || true
  exit 1
fi

echo "GREEN: $TARGET recognizes 'build-planner' in case alternation AND worker pattern"
exit 0
