#!/usr/bin/env bash
# AC-ID: cycle-103-004-list-phase-order-includes-build-planner
# AC-source: scout-report.md AC-2 (lines 321, 353-356)
# Behavioral predicate:
#   scripts/dispatch/list-phase-order.sh must emit "build-planner" between
#   "tdd" and "build" -- under BOTH EVOLVE_USE_PHASE_REGISTRY=1 (registry path)
#   and EVOLVE_USE_PHASE_REGISTRY=0 (hardcoded emit_hardcoded_order fallback).
#
# Mutation spec (cycle-103-004-MUT):
#   Mutant: build-planner inserted AFTER build                     -> must FAIL (order check).
#   Mutant: build-planner absent in EVOLVE_USE_PHASE_REGISTRY=0    -> must FAIL.
#   Mutant: build-planner absent in EVOLVE_USE_PHASE_REGISTRY=1    -> must FAIL.
#
# Bash 3.2 compatible. No GNU-only flags.
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

SCRIPT="scripts/dispatch/list-phase-order.sh"

if [ ! -f "$SCRIPT" ]; then
  echo "RED: $SCRIPT does not exist" >&2
  exit 1
fi

check_order() {
  local registry_flag="$1"
  local output
  output="$(EVOLVE_PROJECT_ROOT="$REPO_ROOT" EVOLVE_USE_PHASE_REGISTRY="$registry_flag" bash "$SCRIPT" 2>/dev/null)"

  if [ -z "$output" ]; then
    echo "RED: $SCRIPT produced empty output with EVOLVE_USE_PHASE_REGISTRY=$registry_flag" >&2
    return 1
  fi

  # Find line numbers (1-based) for tdd, build-planner, build (first occurrence each).
  local n_tdd n_bp n_build
  n_tdd="$(printf '%s\n' "$output" | grep -n '^tdd$' | head -1 | cut -d: -f1)"
  n_bp="$(printf '%s\n' "$output" | grep -n '^build-planner$' | head -1 | cut -d: -f1)"
  n_build="$(printf '%s\n' "$output" | grep -n '^build$' | head -1 | cut -d: -f1)"

  if [ -z "$n_tdd" ]; then
    echo "RED: list-phase-order output missing 'tdd' (REGISTRY=$registry_flag)" >&2
    return 1
  fi
  if [ -z "$n_bp" ]; then
    echo "RED: list-phase-order output missing 'build-planner' (REGISTRY=$registry_flag)" >&2
    printf '%s\n' "$output" >&2
    return 1
  fi
  if [ -z "$n_build" ]; then
    echo "RED: list-phase-order output missing 'build' (REGISTRY=$registry_flag)" >&2
    return 1
  fi

  # Strict ordering: tdd < build-planner < build
  if [ "$n_tdd" -ge "$n_bp" ] || [ "$n_bp" -ge "$n_build" ]; then
    echo "RED: list-phase-order wrong ordering (REGISTRY=$registry_flag): tdd@$n_tdd build-planner@$n_bp build@$n_build" >&2
    printf '%s\n' "$output" >&2
    return 1
  fi

  return 0
}

check_order 1 || exit 1
check_order 0 || exit 1

echo "GREEN: list-phase-order.sh emits tdd < build-planner < build under both REGISTRY=1 and REGISTRY=0"
exit 0
