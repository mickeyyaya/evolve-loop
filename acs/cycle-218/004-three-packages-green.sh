#!/usr/bin/env bash
# ACS cycle-218 / task fix-stale-acs-predicates AC4 — the three previously
# failing packages are fully green (whole-package runs, not just the three
# named tests): no fix may break a sibling test in the same package.
#
# Behavioral: asserts on `go test -race -count=1` EXIT CODE per package via
# the shared assert lib.
set -uo pipefail
TOP=$(git rev-parse --show-toplevel) || { echo "RED: not a git repo" >&2; exit 1; }
. "$TOP/acs/lib/assert.sh"

rc=0
assert_go_test_pass ./acs/cycle89/...    || rc=1
assert_go_test_pass ./acs/cycle100/...   || rc=1
assert_go_test_pass ./internal/setup/... || rc=1

if [ "$rc" -ne 0 ]; then
  echo "RED: at least one of the three packages still fails" >&2
  exit 1
fi
echo "GREEN: cycle89 + cycle100 + internal/setup all pass" >&2
exit 0
