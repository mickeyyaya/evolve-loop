#!/usr/bin/env bash
# AC-ID: cycle-86-regression-suite-passes
# Verify all 5 acs/regression-suite/cycle-86 predicates exit 0 after REPO_ROOT fix.
set -uo pipefail
REPO_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
SUITE_DIR="$REPO_ROOT/acs/regression-suite/cycle-86"

expected_predicates=(
  "pred-auditor-predicate-quality.sh"
  "pred-lint-acs-exists.sh"
  "pred-mutate-grep-only-check.sh"
  "pred-phase-gate-mutation-fail.sh"
  "pred-test-lint-acs-passes.sh"
)

pass=0
fail=0
errors=""

for pred in "${expected_predicates[@]}"; do
  path="$SUITE_DIR/$pred"
  if [ ! -f "$path" ]; then
    errors="${errors}\n  MISSING: $pred"
    fail=$((fail + 1))
    continue
  fi
  out=$(bash "$path" 2>&1)
  rc=$?
  if [ $rc -eq 0 ]; then
    pass=$((pass + 1))
  else
    errors="${errors}\n  FAIL rc=$rc [$pred]: $(echo "$out" | head -3)"
    fail=$((fail + 1))
  fi
done

if [ $fail -gt 0 ]; then
  echo "RED cycle-86-regression-suite-passes: $fail/5 predicates FAILED"
  printf "%b\n" "$errors" >&2
  exit 1
fi
echo "GREEN cycle-86-regression-suite-passes: $pass/5 regression-suite/cycle-86 predicates pass"
exit 0
