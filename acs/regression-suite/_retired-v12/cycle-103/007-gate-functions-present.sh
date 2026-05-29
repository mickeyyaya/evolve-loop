#!/usr/bin/env bash
# AC-ID: cycle-103-007-gate-functions-present
# AC-source: scout-report.md AC-6 (lines 325, 368-374), edit spec lines 210-266
# Behavioral predicate:
#   scripts/lifecycle/phase-gate.sh must declare TWO new gate functions:
#     - gate_tdd_to_build_planner()
#     - gate_build_planner_to_build()
#   AND wire them into the dispatch case switch:
#     - tdd-to-build-planner)
#     - build-planner-to-build)
#
# Both function declarations AND both dispatch entries are required: a function
# can be declared but unreachable if the dispatch switch is not updated.
#
# Mutation spec (cycle-103-007-MUT):
#   Mutant: function declared but missing from dispatch switch -> must FAIL (dispatch grep).
#   Mutant: dispatch entry present but function body absent    -> must FAIL (function grep).
#   Mutant: only one of two functions present                  -> must FAIL.
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

TARGET="scripts/lifecycle/phase-gate.sh"

if [ ! -f "$TARGET" ]; then
  echo "RED: $TARGET does not exist" >&2
  exit 1
fi

assert_grep() {
  local pattern="$1"
  local desc="$2"
  if ! grep -qE "$pattern" "$TARGET"; then
    echo "RED: $TARGET missing $desc (pattern: $pattern)" >&2
    return 1
  fi
  return 0
}

# Function declarations (allow optional whitespace).
assert_grep '^gate_tdd_to_build_planner[[:space:]]*\(\)' \
  "function declaration gate_tdd_to_build_planner()" || exit 1
assert_grep '^gate_build_planner_to_build[[:space:]]*\(\)' \
  "function declaration gate_build_planner_to_build()" || exit 1

# Dispatch case entries (must be on a case-arm line, not in a comment).
# Pattern: leading whitespace + "tdd-to-build-planner)" or "build-planner-to-build)"
assert_grep '^[[:space:]]*tdd-to-build-planner\)' \
  "dispatch case entry 'tdd-to-build-planner)'" || exit 1
assert_grep '^[[:space:]]*build-planner-to-build\)' \
  "dispatch case entry 'build-planner-to-build)'" || exit 1

echo "GREEN: $TARGET declares both gate functions AND both dispatch entries"
exit 0
