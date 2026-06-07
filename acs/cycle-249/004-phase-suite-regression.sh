#!/usr/bin/env bash
# ACS — cycle-249 (both refactor tasks)
# Behavioral regression gate: the FULL phases test suite must pass after
# the BaseCycleContext extraction and the classify migration (intent AC4
# "go test green, no regression" / scout gates rows 2 & 5). Exit code of
# `go test -race` is the authoritative signal (assert.sh, cycle-137 rule).
set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

assert_go_test_pass ./internal/phases/... || exit 1

echo "GREEN: full phases suite passes post-refactor"
exit 0
