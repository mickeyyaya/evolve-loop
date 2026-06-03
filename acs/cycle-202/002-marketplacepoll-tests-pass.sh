#!/usr/bin/env bash
# ACS cycle-202 / AC2 — `go test ./internal/marketplacepoll/...` passes.
#
# Regression guard: the package suite (existing + the four new tests Builder
# adds) must pass with -count=1. Behavioral: asserts on `go test -race` exit
# code via the shared assert lib (the authoritative pass/fail signal — never
# scrapes PASS/ok strings, per the cycle-131/137 lessons).
set -uo pipefail
TOP=$(git rev-parse --show-toplevel) || { echo "RED: not a git repo" >&2; exit 1; }
. "$TOP/acs/lib/assert.sh"

assert_go_test_pass ./internal/marketplacepoll/... || exit 1
exit 0
