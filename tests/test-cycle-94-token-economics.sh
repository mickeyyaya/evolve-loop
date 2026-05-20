#!/usr/bin/env bash
# tests/test-cycle-94-token-economics.sh — runner harness for cycle-94 ACS predicates
#
# Author: tdd-engineer (cycle-94)
# Created: 2026-05-20
#
# Invokes each disposition predicate in acs/cycle-94/ and reports an N/N PASS
# summary. Used by Builder to confirm GREEN before declaring done, and by
# Auditor as the binding test contract for cycle-94 (P1 + P5 + L2).
set -uo pipefail

REPO_ROOT="${WORKTREE_PATH:-${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
CYCLE_DIR="$REPO_ROOT/acs/cycle-94"

if [ ! -d "$CYCLE_DIR" ]; then
  echo "ERROR: $CYCLE_DIR not found" >&2
  exit 2
fi

PASS=0
FAIL=0
FAILED_IDS=""

# Iterate predicates in lexical order so 001 runs before 005.
for pred in "$CYCLE_DIR"/[0-9][0-9][0-9]-*.sh; do
  [ -f "$pred" ] || continue
  name=$(basename "$pred" .sh)
  if bash "$pred"; then
    PASS=$((PASS+1))
  else
    FAIL=$((FAIL+1))
    FAILED_IDS="$FAILED_IDS $name"
  fi
done

TOTAL=$((PASS+FAIL))
if [ "$FAIL" -eq 0 ]; then
  echo "$PASS/$TOTAL PASS"
  exit 0
else
  echo "$PASS/$TOTAL PASS — FAILED:$FAILED_IDS"
  exit 1
fi
