#!/usr/bin/env bash
# AC-ID: cycle-197-001-cycle106-version-pin-fixed
# AC-source: intent.md acceptance_criteria[3] ("stale cycle106 version-pin
#            corrected"); scout-report.md Task 1 gate "go test ./acs/cycle106/...
#            exits 0".
# Behavioral predicate: runs the cycle106 ACS test PACKAGE and asserts it
#   EXITS 0. At RED baseline it FAILS — TestC106_011_BinaryVersionIsV12_1_1
#   pins to strings.Contains(ver,"12.1.1") but the current binary reports a
#   different version ("evolve (devel) ..." / v16.x). After Builder corrects
#   the stale pin (parseable/non-empty check, or a documented t.Skip), the whole
#   package passes. Asserts on the go test EXIT CODE via assert_go_test_pass —
#   never on scraping PASS/ok strings (cycle-137 lesson).
#
# Mutation spec:
#   Mutant: leave the "12.1.1" Contains pin in place -> must FAIL (RED).
#   Mutant: delete TestC106_011 entirely but break another c106 test -> FAIL.
#
# Exit codes: 0 = GREEN, 1 = RED.
set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

assert_go_test_pass ./acs/cycle106/... || exit 1
echo "PASS"; exit 0
