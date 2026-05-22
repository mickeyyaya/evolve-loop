#!/usr/bin/env bash
#
# measure-context-tokens-test.sh — smoke tests for measure-context-tokens.sh.
#
# v8.61.0 Campaign A Cycle A3.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT="$REPO_ROOT/scripts/observability/measure-context-tokens.sh"

PASS=0
FAIL=0
TESTS_TOTAL=0

pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; TESTS_TOTAL=$((TESTS_TOTAL + 1)); }

# --- Test 1: missing argument exits 2 ----------------------------------------
header "Test 1: missing cycle argument exits 2"
set +e
bash "$SCRIPT" >/dev/null 2>&1
rc=$?
set -e
[ "$rc" = "2" ] && pass "missing arg exits 2" || fail_ "expected 2, got $rc"

# --- Test 2: non-numeric argument exits 2 ------------------------------------
header "Test 2: non-numeric cycle argument exits 2"
set +e
bash "$SCRIPT" abc >/dev/null 2>&1
rc=$?
set -e
[ "$rc" = "2" ] && pass "non-numeric arg exits 2" || fail_ "expected 2, got $rc"

# --- Test 3: nonexistent cycle exits 1 ---------------------------------------
header "Test 3: nonexistent cycle exits 1"
set +e
bash "$SCRIPT" 999999 >/dev/null 2>&1
rc=$?
set -e
[ "$rc" = "1" ] && pass "missing cycle exits 1" || fail_ "expected 1, got $rc"

# --- Test 4: existing cycle produces output ----------------------------------
header "Test 4: existing cycle produces table output"
# Find any existing cycle dir to test against.
existing_cycle=$(ls "$REPO_ROOT/.evolve/runs/" 2>/dev/null | grep -E "^cycle-[0-9]+$" | head -1 | sed 's/cycle-//')
if [ -n "$existing_cycle" ]; then
    out=$(bash "$SCRIPT" "$existing_cycle" 2>&1)
    if echo "$out" | grep -q "TOTAL" && echo "$out" | grep -qE "phase[[:space:]]+bedrock"; then
        pass "table output for cycle $existing_cycle"
    else
        fail_ "table output malformed; first 5 lines: $(echo "$out" | head -5)"
    fi
else
    pass "skipped (no existing cycle dirs)"
fi

# --- Test 5: --json output is valid JSON -------------------------------------
header "Test 5: --json output is valid JSON"
if [ -n "$existing_cycle" ] && command -v jq >/dev/null 2>&1; then
    if bash "$SCRIPT" "$existing_cycle" --json 2>/dev/null | jq . >/dev/null 2>&1; then
        pass "--json output validates as JSON"
    else
        fail_ "--json output failed jq parse"
    fi
else
    pass "skipped (no existing cycle or no jq)"
fi

# --- Test 6: JSON contains required top-level keys ---------------------------
header "Test 6: JSON has cycle, phases, total"
if [ -n "$existing_cycle" ] && command -v jq >/dev/null 2>&1; then
    json=$(bash "$SCRIPT" "$existing_cycle" --json 2>/dev/null)
    if echo "$json" | jq -e '.cycle and .phases and .total.bytes and .total.tokens' >/dev/null 2>&1; then
        pass "JSON has cycle, phases, total.bytes, total.tokens"
    else
        fail_ "JSON missing one of: cycle, phases, total.bytes, total.tokens"
    fi
else
    pass "skipped (no existing cycle or no jq)"
fi

# --- Summary -----------------------------------------------------------------
echo
echo "=========================================="
echo "  Total tests: $TESTS_TOTAL"
echo "  Passed:      $PASS"
echo "  Failed:      $FAIL"
echo "=========================================="

[ "$FAIL" = "0" ]
