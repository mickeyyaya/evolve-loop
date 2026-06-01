#!/usr/bin/env bash
# ACS cycle-187 — Task 1 AC-10/AC-11: cyclehealth package comment reflects the
# real signal count (13), not the stale "11".
#
# Behavioral (AC-11): runs the cyclehealth package suite — exit code
# authoritative via assert_go_test_pass. The suite includes
# TestCheck_RunsThirteenSignals, which asserts SignalsRun has exactly 13 entries
# including phase_latency + self_heal_events. That ties the comment's "13" claim
# to the actual signal slice (the semantic anchor for AC-10).
#
# acs-predicate: doc-check
# AC-10 ("no '11-signal' in cyclehealth.go") is inherently a source-COMMENT
# property — there is no runtime behavior to invoke for a comment string, so the
# text check below is a WAIVED grep (the documentation analog of a config-presence
# check). The behavioral weight for the underlying claim is carried by
# TestCheck_RunsThirteenSignals in the package run above, not by this grep.
set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

# AC-11: cyclehealth package compiles + all tests pass (incl. the 13-signal test).
assert_go_test_pass ./internal/cyclehealth/... || exit 1

CH="$(git rev-parse --show-toplevel)/go/internal/cyclehealth/cyclehealth.go"
# AC-10: stale "11-signal" / "The 11 signals" / "runs the 11 signals" must be gone.
if grep -qE '11-signal|The 11 signals|runs the 11 signals' "$CH"; then
  echo "RED: stale 11-signal comment text remains in cyclehealth.go (AC-10)" >&2
  grep -nE '11-signal|The 11 signals|runs the 11 signals' "$CH" >&2
  exit 1
fi
# Positive: the comment must reference 13.
grep -qE '13-signal|The 13 signals|runs the 13 signals' "$CH" \
  || { echo "RED: cyclehealth.go comment does not reference 13 signals (AC-10)" >&2; exit 1; }

echo "PASS: cyclehealth comment reflects 13 signals (AC-10/AC-11)"
exit 0
