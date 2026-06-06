#!/usr/bin/env bash
# ACS — cycle-241 step-5 regression floor: every package this cycle touches
# passes in full (not just the new -run subsets). Pre-existing failures in
# cmd/evolve-fake-cli and internal/skillinventory are environment-isolation
# issues outside step-5 scope (scout-report F4) and are NOT covered here.
# Behavioral: full go test per touched package (exit code is the signal).
set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

rc=0
assert_go_test_pass ./internal/phasecoherence/ || rc=1
assert_go_test_pass ./internal/phases/specrunner/ || rc=1
assert_go_test_pass ./internal/phases/ship/ || rc=1
[ "$rc" -eq 0 ] || exit 1
echo "PASS"
exit 0
