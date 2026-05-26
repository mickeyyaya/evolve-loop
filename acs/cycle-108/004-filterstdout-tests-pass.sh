#!/usr/bin/env bash
# AC-ID: cycle-108-004-filterstdout-tests-pass
# AC-source: scout-report.md AC T2 "go test ./cmd/filter-stdout/... -v exits 0, ≥3 tests pass"
# Behavioral predicate: invokes `go test` and counts PASS lines.
#
# Mutation spec:
#   Mutant: main_test.go deleted → "no test files" (not exit 0 with ≥3 PASS) → fails
#   Mutant: run() removed from main.go → compilation failure → fails
#   Mutant: wrong exit code for no-args → TestRun_NoArgs_ExitsWithCode2 fails
#   Mutant: clean file not written → TestRun_HappyPath_WritesCleanFile fails
#
# Bash 3.2 compatible.
# Exit codes: 0=GREEN, 1=RED
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)"
if [ -z "$REPO_ROOT" ]; then
  echo "RED: not inside a git work tree" >&2; exit 1
fi
cd "$REPO_ROOT/go" || { echo "RED: cd go/ failed" >&2; exit 1; }

output="$(go test ./cmd/filter-stdout/... -v 2>&1)" || {
  echo "RED: go test ./cmd/filter-stdout/... failed"
  echo "$output"
  exit 1
}

pass_count=$(echo "$output" | grep -c '^--- PASS:' || true)
if [ "$pass_count" -lt 3 ]; then
  echo "RED: want ≥3 PASS tests, got $pass_count"
  echo "$output"
  exit 1
fi

fail_count=$(echo "$output" | grep -c '^--- FAIL:' || true)
if [ "$fail_count" -gt 0 ]; then
  echo "RED: $fail_count FAIL lines in output"
  echo "$output"
  exit 1
fi

echo "GREEN: filter-stdout cmd — $pass_count tests PASS, 0 FAIL"
exit 0
