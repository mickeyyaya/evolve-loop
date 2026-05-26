#!/usr/bin/env bash
# AC-ID: cycle-108-001-buildplanner-tests-pass
# AC-source: scout-report.md AC T1 gate "go test ./internal/phases/buildplanner/... -v exits 0, ≥6 tests pass"
# Behavioral predicate: invokes `go test` on the buildplanner package and counts PASS lines.
#
# Mutation spec:
#   Mutant: buildplanner_test.go deleted → fails (no test files)
#   Mutant: ShouldSkip shadow logic removed → TestShouldSkip_ShadowMode fails
#   Mutant: Classify returns PASS on empty → TestClassify_EmptyArtifact fails
#
# Bash 3.2 compatible. No GNU-only flags.
# Exit codes: 0=GREEN, 1=RED
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)"
if [ -z "$REPO_ROOT" ]; then
  echo "RED: not inside a git work tree" >&2; exit 1
fi
cd "$REPO_ROOT/go" || { echo "RED: cd go/ failed" >&2; exit 1; }

output="$(go test ./internal/phases/buildplanner/... -v 2>&1)" || {
  echo "RED: go test ./internal/phases/buildplanner/... failed"
  echo "$output"
  exit 1
}

pass_count=$(echo "$output" | grep -c '^--- PASS:' || true)
if [ "$pass_count" -lt 6 ]; then
  echo "RED: want ≥6 PASS tests, got $pass_count"
  echo "$output"
  exit 1
fi

fail_count=$(echo "$output" | grep -c '^--- FAIL:' || true)
if [ "$fail_count" -gt 0 ]; then
  echo "RED: $fail_count FAIL lines in output"
  echo "$output"
  exit 1
fi

echo "GREEN: buildplanner package — $pass_count tests PASS, 0 FAIL"
exit 0
