#!/usr/bin/env bash
#
# gemini-adapter-test.sh — Contract tests for the hybrid Gemini adapter.
#
# v8.15.0+: gemini.sh is no longer a stub. It is a hybrid shim that delegates
# to scripts/cli_adapters/claude.sh after probing for the claude binary. These
# tests exercise both code paths (claude available; claude forced missing via
# EVOLVE_GEMINI_CLAUDE_PATH=""). When the local machine doesn't have claude
# installed, the "delegation" tests skip cleanly with a PASS — they verify
# behaviour rather than environment.
#
# Usage: bash scripts/gemini-adapter-test.sh
# Exit 0 = all pass; non-zero = failures.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ADAPTER="$REPO_ROOT/scripts/cli_adapters/gemini.sh"
SCOUT_PROFILE="$REPO_ROOT/.evolve/profiles/scout.json"

PASS=0; FAIL=0; TESTS_TOTAL=0
pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; TESTS_TOTAL=$((TESTS_TOTAL + 1)); }

# --- Test 1: adapter file exists --------------------------------------------
header "Test 1: gemini.sh exists"
if [ -f "$ADAPTER" ]; then
    pass "adapter file present"
else
    fail_ "missing $ADAPTER"
fi

# --- Test 2: --probe succeeds when claude is on PATH ------------------------
header "Test 2: --probe succeeds when claude binary is available"
if command -v claude >/dev/null 2>&1; then
    set +e
    bash "$ADAPTER" --probe >/dev/null 2>&1
    rc=$?
    set -e
    if [ "$rc" = "0" ]; then
        pass "probe returned 0 with claude on PATH"
    else
        fail_ "expected rc=0 with claude on PATH, got rc=$rc"
    fi
else
    pass "skipped: claude not on PATH on this machine (probe-availability test n/a)"
fi

# --- Test 3: --probe returns 99 when claude is forced missing ---------------
header "Test 3: --probe exits 99 when claude is forced missing via test seam"
set +e
EVOLVE_GEMINI_CLAUDE_PATH="" bash "$ADAPTER" --probe >/dev/null 2>&1
rc=$?
set -e
if [ "$rc" = "99" ]; then
    pass "probe returned 99 when EVOLVE_GEMINI_CLAUDE_PATH=\"\""
else
    fail_ "expected rc=99 with forced-missing seam, got rc=$rc"
fi

# --- Test 4: error message identifies adapter + install hint ----------------
header "Test 4: 'claude missing' error names adapter + install URL"
set +e
out=$(EVOLVE_GEMINI_CLAUDE_PATH="" bash "$ADAPTER" --probe 2>&1)
set -e
if echo "$out" | grep -q "gemini-adapter" \
   && echo "$out" | grep -qE "claude.ai/code|claude binary not found"; then
    pass "error names adapter + install guidance"
else
    fail_ "error missing required signals; got: $out"
fi

# --- Test 5: error written to stderr, not stdout ----------------------------
header "Test 5: 'claude missing' error routed to stderr"
set +e
stdout_only=$(EVOLVE_GEMINI_CLAUDE_PATH="" bash "$ADAPTER" --probe 2>/dev/null)
stderr_only=$(EVOLVE_GEMINI_CLAUDE_PATH="" bash "$ADAPTER" --probe 2>&1 1>/dev/null)
set -e
if [ -z "$stdout_only" ] && [ -n "$stderr_only" ]; then
    pass "error correctly routed to stderr"
else
    fail_ "stdout='$stdout_only' (should be empty); stderr length=${#stderr_only}"
fi

# --- Test 6: hybrid delegation log line on run mode -------------------------
header "Test 6: delegation log line emitted on run-mode invocation"
if command -v claude >/dev/null 2>&1 && [ -f "$SCOUT_PROFILE" ]; then
    tmpdir=$(mktemp -d)
    echo "test prompt" > "$tmpdir/prompt"
    stderr_log="$tmpdir/adapter-stderr.log"
    set +e
    VALIDATE_ONLY=1 \
        PROFILE_PATH="$SCOUT_PROFILE" \
        RESOLVED_MODEL="sonnet" \
        PROMPT_FILE="$tmpdir/prompt" \
        CYCLE="0" \
        WORKSPACE_PATH="$tmpdir" \
        STDOUT_LOG="$tmpdir/stdout.log" \
        STDERR_LOG="$tmpdir/stderr.log" \
        ARTIFACT_PATH="$tmpdir/artifact.md" \
        bash "$ADAPTER" 2>"$stderr_log" >/dev/null
    rc=$?
    set -e
    if grep -q "hybrid-mode: delegating to claude.sh" "$stderr_log" 2>/dev/null; then
        pass "delegation log line emitted (claude.sh exit code: $rc)"
    else
        fail_ "delegation log missing. stderr captured: $(cat "$stderr_log")"
    fi
    rm -rf "$tmpdir"
else
    pass "skipped: claude not on PATH or scout profile missing (delegation test n/a)"
fi

# --- Test 7: --probe ignores PROFILE_PATH and other env vars ----------------
header "Test 7: --probe doesn't require run-mode env vars"
set +e
PROFILE_PATH="" RESOLVED_MODEL="" PROMPT_FILE="" \
    bash "$ADAPTER" --probe >/dev/null 2>&1
rc=$?
set -e
# rc should be 0 (claude found) or 99 (claude missing) — not a "missing env" 127 or similar
if [ "$rc" = "0" ] || [ "$rc" = "99" ]; then
    pass "probe rc=$rc (decoupled from run-mode env contract)"
else
    fail_ "probe should not require run-mode env; got rc=$rc"
fi

# --- Test 8: claude.sh adapter still present (sanity for delegation target) -
header "Test 8: claude.sh delegation target exists"
CLAUDE_SH="$REPO_ROOT/scripts/cli_adapters/claude.sh"
if [ -x "$CLAUDE_SH" ] || [ -f "$CLAUDE_SH" ]; then
    pass "claude.sh exists (delegation will resolve)"
else
    fail_ "claude.sh missing — gemini.sh hybrid would fail at exec step"
fi

echo
echo "=========================================="
echo "  Total tests: $TESTS_TOTAL"
echo "  Passed:      $PASS"
echo "  Failed:      $FAIL"
echo "=========================================="
[ "$FAIL" = "0" ] && exit 0 || exit 1
