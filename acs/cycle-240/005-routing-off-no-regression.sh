#!/usr/bin/env bash
# ACS — cycle-240 constraint: behavior under EVOLVE_DYNAMIC_ROUTING=off (and
# the whole static path) is unchanged — the FULL router + core suites pass,
# including every pre-existing StageOff/static scenario. Behavioral: full
# package test runs; exit code is the signal.
set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

assert_go_test_pass ./internal/router/... || exit 1
assert_go_test_pass ./internal/core/... || exit 1
echo "PASS"
exit 0
