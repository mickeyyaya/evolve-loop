#!/usr/bin/env bash
#
# codex-adapter-test.sh — Stub-contract tests for scripts/cli_adapters/codex.sh.
#
# v8.13.7: codex.sh is intentionally an unimplemented stub that exits 99 with a
# helpful error message directing operators back to the 'claude' provider. These
# tests pin the stub contract: if anyone makes codex.sh "work" by accident
# (e.g., changing the exit code or removing the WARN message), CI catches it
# and the change must come with a real adapter implementation + full test suite.
#
# When codex.sh becomes a real adapter, REPLACE this file with one that mirrors
# claude-adapter-test.sh structure (VALIDATE_ONLY mode, profile loading, budget
# precedence, sandbox profile generation, etc.).
#
# Usage: bash scripts/codex-adapter-test.sh
# Exit 0 = all pass; non-zero = failures.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ADAPTER="$REPO_ROOT/scripts/cli_adapters/codex.sh"

PASS=0; FAIL=0; TESTS_TOTAL=0
pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; TESTS_TOTAL=$((TESTS_TOTAL + 1)); }

# === Test 1: file exists and is executable ===================================
header "Test 1: codex.sh exists"
if [ -f "$ADAPTER" ]; then
    pass "adapter file present"
else
    fail_ "missing $ADAPTER"
fi

# === Test 2: stub exits with code 99 (loud failure) ==========================
# Rationale: code 99 distinguishes "not implemented" from generic CLI errors
# (1, 2, 127). A silent exit 0 or generic 1 would let evolve-loop continue
# without provider isolation.
header "Test 2: stub exits 99 (not 0, not 1, not 127)"
set +e
out=$(bash "$ADAPTER" 2>&1)
rc=$?
set -e
if [ "$rc" = "99" ]; then
    pass "exit code 99 as documented"
else
    fail_ "expected rc=99, got rc=$rc"
fi

# === Test 3: stub error message names the file and the fix ===================
# Operators must know (a) it's the codex adapter that failed, (b) how to recover.
header "Test 3: error message identifies adapter + recovery path"
if echo "$out" | grep -q "codex.sh:" \
   && echo "$out" | grep -qiE "not (yet )?implement"; then
    pass "error names adapter + 'not implemented'"
else
    fail_ "error message missing required signal: $out"
fi

# === Test 4: stub points operator at claude provider as workaround ===========
header "Test 4: error message tells operator to use 'claude' provider"
if echo "$out" | grep -q '"claude"'; then
    pass "error directs to claude provider"
else
    fail_ "error missing claude-provider workaround hint"
fi

# === Test 5: stub error goes to stderr (not stdout) ==========================
# stdout is reserved for the adapter's normal output (when implemented). The
# stub's error message must go to stderr so it doesn't pollute parsed output.
header "Test 5: error written to stderr, not stdout"
set +e
stdout_only=$(bash "$ADAPTER" 2>/dev/null)
stderr_only=$(bash "$ADAPTER" 2>&1 1>/dev/null)
rc=$?
set -e
if [ -z "$stdout_only" ] && [ -n "$stderr_only" ]; then
    pass "error correctly routed to stderr"
else
    fail_ "stdout='$stdout_only' (should be empty); stderr length=${#stderr_only}"
fi

# === Test 6: implementation guidance present in stub =========================
# The stub doubles as docs for whoever implements it. Make sure the guidance
# block names the key fields any real adapter must wire up.
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

# === Summary ==================================================================
echo
echo "=========================================="
echo "  Total tests: $TESTS_TOTAL"
echo "  Passed:      $PASS"
echo "  Failed:      $FAIL"
echo "=========================================="
[ "$FAIL" = "0" ] && exit 0 || exit 1
