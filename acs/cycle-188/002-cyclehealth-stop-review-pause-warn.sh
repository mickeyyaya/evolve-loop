#!/usr/bin/env bash
# ACS cycle-188 — Task 1 AC4: checkSelfHealEvents treats a kind=stop_review
# ledger entry with action=pause as a self_heal_events WARN (naming the phase,
# never fatal), while action=extend is healthy and emits nothing.
#
# BEHAVIORAL: runs the cyclehealth tests as a subprocess; pass/fail is the
# go-test EXIT CODE. The pause test asserts exactly one WARN; the extend test
# is the anti-no-op guard (an impl that warned on every stop_review fails it).
set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

assert_go_test_pass ./internal/cyclehealth/... 'TestCheck_SelfHealEvents_StopReview' || exit 1
echo "PASS: stop_review/pause→WARN, stop_review/extend→no-anomaly"
exit 0
