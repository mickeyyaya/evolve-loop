#!/usr/bin/env bash
# tests/test-cherry-pick-rescue-235.sh — cycle-236 TDD contract driver.
# Task: re-land rescue/cycle-235-audited (e565834 + 6f2e1af) onto main via
# cherry-pick, skipping scaffold 568085c. Runs the three cycle-236 ACS
# predicates; RED before the cherry-pick, GREEN after Builder lands it.
set -uo pipefail
TOP=$(git rev-parse --show-toplevel)
cd "$TOP"

PASS=0; FAIL=0
run_predicate() {
  local label="$1" script="$2"
  if bash "$script"; then
    echo "PASS: $label"
    PASS=$((PASS+1))
  else
    echo "FAIL: $label"
    FAIL=$((FAIL+1))
  fi
}

run_predicate "AC1 rescue commits landed, zero drift, scaffold skipped" \
  acs/cycle-236/001-rescue-parity-landed.sh
run_predicate "AC3a named supervision-tree/ship/audit-leak tests green" \
  acs/cycle-236/002-supervision-tree-tests-green.sh
run_predicate "AC3b whole-suite regression + build + gofmt -s CI parity" \
  acs/cycle-236/003-regression-ci-parity.sh

echo ""
echo "Results: $PASS PASS, $FAIL FAIL"
[ "$FAIL" -eq 0 ]
