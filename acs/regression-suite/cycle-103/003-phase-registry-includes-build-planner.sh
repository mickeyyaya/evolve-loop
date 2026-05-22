#!/usr/bin/env bash
# AC-ID: cycle-103-003-phase-registry-includes-build-planner
# AC-source: scout-report.md AC-3 (lines 322, 348-351), registry spec lines 121-149
# Behavioral predicate:
#   docs/architecture/phase-registry.json must contain a phase entry with
#   name == "build-planner", and the TDD phase's gate_out must equal
#   "gate_tdd_to_build_planner" (replacing the previous "gate_discover_to_build").
#
# Additionally verifies the new phase's gates wire correctly:
#   - build-planner.gate_in  == "gate_tdd_to_build_planner"
#   - build-planner.gate_out == "gate_build_planner_to_build"
#   - build-planner.enable_var == "EVOLVE_BUILD_PLANNER"
#
# Mutation spec (cycle-103-003-MUT):
#   Mutant: tdd.gate_out still "gate_discover_to_build"            -> must FAIL.
#   Mutant: build-planner entry absent                              -> must FAIL.
#   Mutant: build-planner.enable_var missing or wrong               -> must FAIL.
#
# Bash 3.2 compatible. Depends on jq.
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

REGISTRY="docs/architecture/phase-registry.json"

if [ ! -f "$REGISTRY" ]; then
  echo "RED: $REGISTRY does not exist" >&2
  exit 1
fi

if ! jq -e . "$REGISTRY" >/dev/null 2>&1; then
  echo "RED: $REGISTRY is not valid JSON" >&2
  exit 1
fi

# 1. build-planner phase entry exists
if ! jq -e '.phases[] | select(.name == "build-planner")' "$REGISTRY" >/dev/null 2>&1; then
  echo "RED: $REGISTRY has no phase named 'build-planner'" >&2
  exit 1
fi

# 2. tdd.gate_out is gate_tdd_to_build_planner
tdd_gate_out="$(jq -r '.phases[] | select(.name == "tdd") | .gate_out' "$REGISTRY" 2>/dev/null)"
if [ "$tdd_gate_out" != "gate_tdd_to_build_planner" ]; then
  echo "RED: $REGISTRY tdd.gate_out='$tdd_gate_out', expected 'gate_tdd_to_build_planner'" >&2
  exit 1
fi

# 3. build-planner.gate_in
bp_gate_in="$(jq -r '.phases[] | select(.name == "build-planner") | .gate_in' "$REGISTRY" 2>/dev/null)"
if [ "$bp_gate_in" != "gate_tdd_to_build_planner" ]; then
  echo "RED: $REGISTRY build-planner.gate_in='$bp_gate_in', expected 'gate_tdd_to_build_planner'" >&2
  exit 1
fi

# 4. build-planner.gate_out
bp_gate_out="$(jq -r '.phases[] | select(.name == "build-planner") | .gate_out' "$REGISTRY" 2>/dev/null)"
if [ "$bp_gate_out" != "gate_build_planner_to_build" ]; then
  echo "RED: $REGISTRY build-planner.gate_out='$bp_gate_out', expected 'gate_build_planner_to_build'" >&2
  exit 1
fi

# 5. enable_var
bp_enable="$(jq -r '.phases[] | select(.name == "build-planner") | .enable_var' "$REGISTRY" 2>/dev/null)"
if [ "$bp_enable" != "EVOLVE_BUILD_PLANNER" ]; then
  echo "RED: $REGISTRY build-planner.enable_var='$bp_enable', expected 'EVOLVE_BUILD_PLANNER'" >&2
  exit 1
fi

echo "GREEN: phase-registry has build-planner; tdd.gate_out and gate wiring correct"
exit 0
