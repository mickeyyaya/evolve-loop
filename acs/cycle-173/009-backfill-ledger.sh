#!/usr/bin/env bash
# AC-ID: cycle-173-009-backfill-ledger
# AC-source: scout-report.md T2 "kind=backfill ledger entry with Role=phase-name
#   after TryExtract success" + "No backfill ledger entry in normal cycle"
#
# Behavioral predicate. Drives the three backfill tests (positive entry, role
# attribution, and the negative disabled-path axis) and asserts on the go test
# EXIT CODE via acs/lib/assert.sh, with a -list discoverability guard per test
# (cycle-172 trap). RED until Builder appends the kind=backfill ledger entry on
# backfill.TryExtract success; GREEN after.
#
# Exit: 0 = GREEN, 1 = RED. Bash 3.2 compatible.
set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

PKG="./internal/core/..."
DIR="$(acs_go_module_dir)"

TESTS="
TestOrchestrator_Backfill_LedgerEntry
TestOrchestrator_Backfill_LedgerRole
TestOrchestrator_Backfill_NoLedgerEntryWhenDisabled
"

for T in $TESTS; do
  if ! (cd "$DIR" && go test -list "^${T}$" "$PKG" 2>/dev/null) | grep -qx "$T"; then
    echo "RED: $T not discoverable by 'go test -list' (cycle-172 trap)" >&2
    exit 1
  fi
  assert_go_test_pass "$PKG" "^${T}$" || exit 1
done

echo "PASS: backfill ledger entry (positive + role + negative-disabled) all green"
exit 0
