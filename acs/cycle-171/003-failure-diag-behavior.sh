#!/usr/bin/env bash
# ACS cycle-171 T2 — <phase>-failure-diag.json written on phase abort.
# Behavioral: drives a cycle that aborts on exhausted retries and asserts the
# diag file exists with phase / exit_code=81 / non-empty error_message, AND that
# a PASS cycle writes none (negative axis). The 'TestFailureDiag' regex runs both.
set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"
assert_go_test_pass ./internal/core/... 'TestFailureDiag' || exit 1
