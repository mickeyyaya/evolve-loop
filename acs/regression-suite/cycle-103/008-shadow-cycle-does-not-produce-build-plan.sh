#!/usr/bin/env bash
# AC-ID: cycle-103-008-shadow-cycle-does-not-produce-build-plan
# AC-source: scout-report.md AC-7 (lines 326, 376-379)
# Behavioral predicate (SHADOW invariant):
#   With EVOLVE_BUILD_PLANNER unset (default :- 0), cycle 103 must NOT
#   produce .evolve/runs/cycle-103/build-plan.md. This is the load-bearing
#   shadow-cycle invariant: cycle 1 wires infrastructure but the persona
#   does NOT run.
#
# This predicate ALSO defensively checks that the orchestrator-reference's
# build-planner conditional uses :- 0 (not :- 1), since the wrong default
# is the easiest way to silently violate the shadow invariant.
#
# Mutation spec (cycle-103-008-MUT):
#   Mutant: orchestrator default :- 1                          -> would produce build-plan.md -> must FAIL.
#   Mutant: build-plan.md exists in cycle-103/                 -> must FAIL.
#   Mutant: orchestrator-reference missing EVOLVE_BUILD_PLANNER gate at all -> must FAIL.
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

# 1. Shadow invariant: build-plan.md must NOT exist this cycle.
if [ -f "$BUILD_PLAN" ]; then
  echo "RED: $BUILD_PLAN exists -- shadow invariant violated (EVOLVE_BUILD_PLANNER default must be 0)" >&2
  exit 1
fi

# 2. Defensive: orchestrator-reference must gate on EVOLVE_BUILD_PLANNER with default :- 0
ORCH_REF="agents/evolve-orchestrator-reference.md"

if [ ! -f "$ORCH_REF" ]; then
  echo "RED: $ORCH_REF does not exist (cannot verify default-off gate)" >&2
  exit 1
fi

# Must mention EVOLVE_BUILD_PLANNER
if ! grep -q 'EVOLVE_BUILD_PLANNER' "$ORCH_REF"; then
  echo "RED: $ORCH_REF does not mention EVOLVE_BUILD_PLANNER" >&2
  exit 1
fi

# The conditional must default to 0 (shadow-cycle invariant).
# Accept either ${EVOLVE_BUILD_PLANNER:-0} or ${EVOLVE_BUILD_PLANNER:- 0} (whitespace tolerant).
if ! grep -Eq '\$\{EVOLVE_BUILD_PLANNER:-[[:space:]]*0\}' "$ORCH_REF"; then
  echo "RED: $ORCH_REF EVOLVE_BUILD_PLANNER must default to 0 (\${EVOLVE_BUILD_PLANNER:-0})" >&2
  echo "Found these EVOLVE_BUILD_PLANNER lines instead:" >&2
  grep -n 'EVOLVE_BUILD_PLANNER' "$ORCH_REF" >&2 || true
  exit 1
fi

# Must NOT default to 1 anywhere.
if grep -Eq '\$\{EVOLVE_BUILD_PLANNER:-[[:space:]]*1\}' "$ORCH_REF"; then
  echo "RED: $ORCH_REF has \${EVOLVE_BUILD_PLANNER:-1} -- shadow invariant violated" >&2
  grep -n 'EVOLVE_BUILD_PLANNER:-1' "$ORCH_REF" >&2 || true
  exit 1
fi

echo "GREEN: shadow invariant holds (no build-plan.md; orchestrator defaults EVOLVE_BUILD_PLANNER to 0)"
exit 0
