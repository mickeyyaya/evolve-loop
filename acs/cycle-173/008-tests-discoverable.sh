#!/usr/bin/env bash
# AC-ID: cycle-173-008-tests-discoverable
# AC-source: scout-report.md T1 AC "all new named tests discoverable by -run AND
#   -list" + T2 AC "3 new named tests discoverable by -run AND -list" + the
#   scout-report Reflection: "ACS predicates should verify [no tests to run] is
#   absent in addition to checking the exit code."
#
# THE cycle-172 REGRESSION GUARD. Cycle 172 FAILED because its build report named
# test functions that were never written: `go test -run <name>` matched nothing,
# printed "[no tests to run]", and exited 0 — so the ACS predicate passed falsely.
# This predicate makes that failure mode impossible to repeat: every named test
# (T1 + T2) MUST appear in `go test -list`, and a `-run -v` invocation across all
# of them MUST execute at least one and MUST NOT print "[no tests to run]".
#
# Exit: 0 = GREEN, 1 = RED. Bash 3.2 compatible.
set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

PKG="./internal/core/..."
DIR="$(acs_go_module_dir)"

# Every test this cycle promises. A bare-minimum subset would let a partial
# implementation slip; the full list is the contract.
TESTS="
TestIsTransientBridgeError
TestOrchestrator_RetryOnTransientExit
TestOrchestrator_NonCanonicalVerdictRetry
TestTransientRetry_LedgerEntry
TestOrchestrator_NonTransientError_NoRetry
TestTransientRetry_Exhausted_WritesFailureDiag
TestFAILVerdict_NotRetried
TestOrchestrator_Backfill_LedgerEntry
TestOrchestrator_Backfill_LedgerRole
TestOrchestrator_Backfill_NoLedgerEntryWhenDisabled
"

rc=0
for T in $TESTS; do
  if ! (cd "$DIR" && go test -list "^${T}$" "$PKG" 2>/dev/null) | grep -qx "$T"; then
    echo "RED: $T not discoverable by 'go test -list' (the cycle-172 missing-test trap)" >&2
    rc=1
  fi
done
[ "$rc" -eq 0 ] || exit 1

# Build an alternation regex of every test and run them verbosely; assert the
# output never contains the [no tests to run] sentinel and at least one ran.
ALT="$(echo "$TESTS" | grep -v '^$' | paste -sd'|' -)"
OUT="$(cd "$DIR" && go test -run "^(${ALT})$" -v -count=1 "$PKG" 2>&1)"
if echo "$OUT" | grep -q 'no tests to run'; then
  echo "RED: 'no tests to run' present — a named test resolved to zero matches (cycle-172 trap)" >&2
  echo "$OUT" | grep 'no tests to run' >&2
  exit 1
fi
if ! echo "$OUT" | grep -qE '^(ok|=== RUN)'; then
  echo "RED: no test actually executed (expected RUN/ok lines)" >&2
  echo "$OUT" | tail -5 >&2
  exit 1
fi

echo "PASS: all 10 named tests are -list-discoverable and at least one executed; no '[no tests to run]'"
exit 0
