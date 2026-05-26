#!/usr/bin/env bash
# AC-ID: cycle-108-005-global-tests-clean
# AC-source: scout-report.md AC "go test ./... exits 0, zero FAIL lines"
# Behavioral predicate: runs full test suite and asserts no FAIL lines.
#
# Mutation spec:
#   Mutant: any package broken → go test fails → RED
#   Mutant: any test FAIL line → RED
#
# Bash 3.2 compatible.
# Exit codes: 0=GREEN, 1=RED
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)"
if [ -z "$REPO_ROOT" ]; then
  echo "RED: not inside a git work tree" >&2; exit 1
fi
cd "$REPO_ROOT/go" || { echo "RED: cd go/ failed" >&2; exit 1; }

output="$(go test ./... 2>&1)" || {
  echo "RED: go test ./... failed (non-zero exit)"
  echo "$output"
  exit 1
}

fail_count=$(echo "$output" | grep -c '^--- FAIL:' || true)
if [ "$fail_count" -gt 0 ]; then
  echo "RED: $fail_count FAIL lines in go test ./..."
  echo "$output" | grep '^--- FAIL:' >&2
  exit 1
fi

echo "GREEN: go test ./... — 0 FAIL lines"
exit 0
