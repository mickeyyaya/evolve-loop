#!/usr/bin/env bash
# AC-ID: cycle-103-008-shadow-cycle-does-not-produce-build-plan
# AC-source: scout-report.md AC-7 (lines 326, 376-379)
# Behavioral predicate (ARTIFACT invariant — updated v10.20):
#   Cycle 103 ran as shadow (EVOLVE_BUILD_PLANNER unset/0) and must NOT
#   have produced .evolve/runs/cycle-103/build-plan.md.
#
# NOTE (v10.20 update, cycle-104): The orchestrator default :- 0 check was
# removed from this predicate. In v10.20 (cycle-104) the default flipped to
# :- 1 (advisory mode). The orchestrator-default invariant is now enforced by
# acs/cycle-104/001-orchestrator-default-advisory.sh. This predicate retains
# the cycle-103 artifact-absence check, which remains valid indefinitely
# (cycle-103 ran before advisory mode was enabled).
#
# Mutation spec (cycle-103-008-MUT, updated):
#   Mutant: build-plan.md exists in cycle-103/                 -> must FAIL.
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

BUILD_PLAN=".evolve/runs/cycle-103/build-plan.md"

# Shadow artifact invariant: build-plan.md must NOT exist for cycle-103.
if [ -f "$BUILD_PLAN" ]; then
  echo "RED: $BUILD_PLAN exists -- cycle-103 shadow invariant violated" >&2
  exit 1
fi

echo "GREEN: shadow artifact invariant holds (no .evolve/runs/cycle-103/build-plan.md)"
exit 0
