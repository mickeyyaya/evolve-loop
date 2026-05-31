#!/usr/bin/env bash
# AC-ID: cycle-173-005-nontransient-no-retry
# AC-source: scout-report.md T1 transient-bridge-retry — NEGATIVE: non-transient exit 2 aborts attempt 1, no retry
#
# Behavioral predicate. Asserts on the go test EXIT CODE via acs/lib/assert.sh
# (never scrapes PASS lines — cycle-131/137 footgun) AND guards discoverability
# so a missing/renamed test is RED, not a false PASS (cycle-172 [no tests to run]
# trap). RED until Builder implements the transient-retry logic; GREEN after.
#
# Exit: 0 = GREEN, 1 = RED. Bash 3.2 compatible.
set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

PKG="./internal/core/..."
T="TestOrchestrator_NonTransientError_NoRetry"
DIR="$(acs_go_module_dir)"

# Discoverability guard: exact top-level test name must be listed.
if ! (cd "$DIR" && go test -list "^${T}$" "$PKG" 2>/dev/null) | grep -qx "$T"; then
  echo "RED: $T not discoverable by 'go test -list' (missing/renamed — the cycle-172 trap)" >&2
  exit 1
fi

assert_go_test_pass "$PKG" "^${T}$" || exit 1
echo "PASS: $T discoverable and green"
exit 0
