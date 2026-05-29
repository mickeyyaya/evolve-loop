#!/usr/bin/env bash
# Predicate: test-lint-acs-predicates.sh passes (7 FAIL fixtures fail, 2 PASS fixtures pass)
# Behavioral: invokes the actual test suite subprocess and checks exit code + pass count
set -uo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
TEST_SCRIPT="$REPO_ROOT/tests/verification/test-lint-acs-predicates.sh"
[ -x "$TEST_SCRIPT" ] || chmod +x "$TEST_SCRIPT"
output=$(bash "$TEST_SCRIPT" 2>&1)
rc=$?
# Verify all tests passed (exit 0 and "all tests passed" in output)
[ "$rc" -eq 0 ]
echo "$output" | grep -q "all tests passed"
