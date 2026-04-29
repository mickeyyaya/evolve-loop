#!/usr/bin/env bash
#
# gemini-adapter-test.sh — Stub-contract tests for scripts/cli_adapters/gemini.sh.
#
# v8.13.7: gemini.sh is intentionally an unimplemented stub that exits 99 with a
# helpful error message directing operators back to the 'claude' provider.
# Mirrors codex-adapter-test.sh — see that file for the rationale and replacement
# instructions when a real adapter is built.
#
# Usage: bash scripts/gemini-adapter-test.sh
# Exit 0 = all pass; non-zero = failures.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ADAPTER="$REPO_ROOT/scripts/cli_adapters/gemini.sh"

PASS=0; FAIL=0; TESTS_TOTAL=0
pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; TESTS_TOTAL=$((TESTS_TOTAL + 1)); }

header "Test 1: gemini.sh exists"
if [ -f "$ADAPTER" ]; then
    pass "adapter file present"
else
    fail_ "missing $ADAPTER"
fi

header "Test 2: stub exits 99"
set +e
out=$(bash "$ADAPTER" 2>&1)
rc=$?
set -e
if [ "$rc" = "99" ]; then
    pass "exit code 99 as documented"
else
    fail_ "expected rc=99, got rc=$rc"
fi

header "Test 3: error message identifies adapter + 'not implemented'"
if echo "$out" | grep -q "gemini.sh:" \
   && echo "$out" | grep -qiE "not (yet )?implement"; then
    pass "error names adapter + 'not implemented'"
else
    fail_ "error message missing required signal: $out"
fi

header "Test 4: error message tells operator to use 'claude' provider"
if echo "$out" | grep -q '"claude"'; then
    pass "error directs to claude provider"
else
    fail_ "error missing claude-provider workaround hint"
fi

header "Test 5: error written to stderr, not stdout"
set +e
stdout_only=$(bash "$ADAPTER" 2>/dev/null)
stderr_only=$(bash "$ADAPTER" 2>&1 1>/dev/null)
set -e
if [ -z "$stdout_only" ] && [ -n "$stderr_only" ]; then
    pass "error correctly routed to stderr"
else
    fail_ "stdout='$stdout_only' (should be empty); stderr length=${#stderr_only}"
fi

header "Test 6: implementation guidance lists key fields"
missing=()
for keyword in "allowed_tools" "add_dir" "max_budget_usd" "STDOUT_LOG"; do
    if ! echo "$out" | grep -q "$keyword"; then
        missing+=("$keyword")
    fi
done
if [ "${#missing[@]}" = "0" ]; then
    pass "all guidance keywords present"
else
    fail_ "missing guidance keywords: ${missing[*]}"
fi

echo
echo "=========================================="
echo "  Total tests: $TESTS_TOTAL"
echo "  Passed:      $PASS"
echo "  Failed:      $FAIL"
echo "=========================================="
[ "$FAIL" = "0" ] && exit 0 || exit 1
