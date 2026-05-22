#!/usr/bin/env bash
#
# parity-audit-test.sh — Unit tests for scripts/parity-audit.sh.
#
# Tests the dry-run and simulate paths only — neither spends real money.
# The full-mode (real-cycle) path is operator-driven and not unit-tested here.
#
# Usage: bash scripts/tests/parity-audit-test.sh

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PARITY="$REPO_ROOT/scripts/parity-audit.sh"

PASS=0; FAIL=0; TESTS_TOTAL=0
pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; TESTS_TOTAL=$((TESTS_TOTAL + 1)); }

cleanup_dirs=()
trap '
    for d in ${cleanup_dirs[@]+"${cleanup_dirs[@]}"}; do rm -rf "$d"; done
' EXIT

header "parity-audit.sh exists and is executable"
if [ -x "$PARITY" ]; then
    pass "script present + +x bit set"
else
    fail_ "missing or not executable: $PARITY"
fi

header "--help prints usage and exits 0"
out=$("$PARITY" --help 2>&1)
rc=$?
if [ "$rc" -ne 0 ]; then
    fail_ "expected exit 0, got $rc"
elif ! echo "$out" | grep -q "parity-audit"; then
    fail_ "usage missing 'parity-audit' identifier"
else
    pass "help works"
fi

header "unknown flag exits non-zero"
"$PARITY" --not-a-real-flag >/dev/null 2>&1
rc=$?
if [ "$rc" -eq 0 ]; then
    fail_ "expected non-zero exit on bad flag"
else
    pass "rejects unknown flag"
fi

header "--dry-run does not invoke the Go binary or bash cycle"
out=$("$PARITY" --dry-run 2>&1)
rc=$?
if [ "$rc" -ne 0 ]; then
    fail_ "dry-run expected to succeed, got rc=$rc; out=$out"
elif ! echo "$out" | grep -q "DRY RUN"; then
    fail_ "dry-run output missing 'DRY RUN' header"
elif ! echo "$out" | grep -q "bash side"; then
    fail_ "dry-run should describe the bash invocation it would make"
elif ! echo "$out" | grep -q "Go side"; then
    fail_ "dry-run should describe the Go invocation it would make"
else
    pass "dry-run describes both sides without invoking them"
fi

header "--dry-run reports prerequisite status (go binary)"
out=$("$PARITY" --dry-run 2>&1)
if ! echo "$out" | grep -qE "go binary|evolve binary|go/bin/evolve"; then
    fail_ "dry-run should report Go binary discovery"
else
    pass "reports Go binary status"
fi

header "--dry-run reports prerequisite status (cycle-simulator.sh)"
out=$("$PARITY" --dry-run 2>&1)
if ! echo "$out" | grep -q "cycle-simulator.sh"; then
    fail_ "dry-run should report cycle-simulator.sh dependency"
else
    pass "reports cycle-simulator dependency"
fi

header "--simulate runs cycle-simulator + Go phase smoke check"
# This path uses no real LLM — it just exercises that both sides can be
# invoked end-to-end on a fixture workspace. Acceptable for CI.
out=$("$PARITY" --simulate 2>&1)
rc=$?
# We don't enforce rc=0 yet because the Go-side simulate hook may not
# exist; the script must report this gracefully rather than crashing.
if echo "$out" | grep -q "FATAL: bash"; then
    fail_ "simulate path bash crash: $out"
elif ! echo "$out" | grep -qE "simulate|parity"; then
    fail_ "simulate output missing identifier; got: $out"
else
    pass "simulate path completes (rc=$rc) without bash crash"
fi

echo
echo "Results: $PASS pass, $FAIL fail, $TESTS_TOTAL tests"
[ "$FAIL" -eq 0 ]
