#!/usr/bin/env bash
# ACS cycle-202 / AC5 — at least one new test is a negative / error-path case.
#
# The named negative test exercises DefaultReleaseSh's failure leg (script
# present but exits non-zero -> wrapped error). Two checks, both load-bearing:
#
#   1. EXISTENCE (grep the test deliverable): `go test -run <missing>` exits 0
#      with "no tests to run", so the run ALONE cannot prove the test exists.
#      The grep is on the test file (the deliverable), not on the source under
#      test for a magic string — so this is not the cycle-85 grep-only mode.
#   2. CORRECTNESS (assert_go_test_pass): the test must actually pass, asserted
#      on `go test` exit code.
#
# RED at baseline (the negative test does not exist yet); GREEN once Builder
# adds TestDefaultReleaseSh_ScriptExitsNonZero_ReturnsError.
set -uo pipefail
TOP=$(git rev-parse --show-toplevel) || { echo "RED: not a git repo" >&2; exit 1; }
. "$TOP/acs/lib/assert.sh"

TESTFILE="$TOP/go/internal/marketplacepoll/marketplacepoll_test.go"
NEG='TestDefaultReleaseSh_ScriptExitsNonZero_ReturnsError'

if ! grep -q "func ${NEG}(" "$TESTFILE"; then
  echo "RED: negative error-path test ${NEG} not found in ${TESTFILE}" >&2
  exit 1
fi

assert_go_test_pass ./internal/marketplacepoll/... "^${NEG}$" || exit 1
echo "GREEN: negative error-path test present and passing" >&2
exit 0
