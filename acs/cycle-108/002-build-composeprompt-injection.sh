#!/usr/bin/env bash
# AC-ID: cycle-108-002-build-composeprompt-injection
# AC-source: scout-report.md AC T1 gate "go test ./internal/phases/build/... -v -run TestComposePrompt exits 0"
# Behavioral predicate: runs TestComposePrompt tests including injection variants.
#
# Mutation spec:
#   Mutant: build-plan.md injection removed → TestComposePrompt_InjectsBuildPlanWhenPresent fails
#   Mutant: injection always runs → TestComposePrompt_SkipsBuildPlanWhenAbsent fails
#   Mutant: wrong section heading → TestComposePrompt_InjectsBuildPlanWhenPresent fails
#
# Bash 3.2 compatible.
# Exit codes: 0=GREEN, 1=RED
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)"
if [ -z "$REPO_ROOT" ]; then
  echo "RED: not inside a git work tree" >&2; exit 1
fi
cd "$REPO_ROOT/go" || { echo "RED: cd go/ failed" >&2; exit 1; }

output="$(go test ./internal/phases/build/... -v -run TestComposePrompt 2>&1)" || {
  echo "RED: go test ./internal/phases/build/... -run TestComposePrompt failed"
  echo "$output"
  exit 1
}

if echo "$output" | grep -q '^--- FAIL:'; then
  echo "RED: FAIL lines in TestComposePrompt run"
  echo "$output"
  exit 1
fi

# Specifically require both injection tests to appear
for test_name in TestComposePrompt_InjectsBuildPlanWhenPresent TestComposePrompt_SkipsBuildPlanWhenAbsent; do
  if ! echo "$output" | grep -qF "PASS: $test_name"; then
    echo "RED: $test_name not PASS (may be missing from build_test.go)"
    echo "$output"
    exit 1
  fi
done

echo "GREEN: TestComposePrompt* tests all pass including injection variants"
exit 0
