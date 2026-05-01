#!/usr/bin/env bash
#
# fanout-dispatch-test.sh — Unit tests for scripts/fanout-dispatch.sh.
#
# fanout-dispatch.sh is the parallel-worker dispatcher used by Sprint 1
# of the swarm architecture (see plans/does-the-flow-allow-jaunty-hummingbird.md).
# It runs N worker commands concurrently (bounded by EVOLVE_FANOUT_CONCURRENCY,
# default 4), enforces a per-worker timeout (EVOLVE_FANOUT_TIMEOUT, default 600s),
# captures each worker's exit code + duration to a TSV file, and uses WAIT-ALL
# semantics (every worker runs to completion or timeout regardless of others'
# failures).
#
# Tests cover:
#   1. script exists and is executable
#   2. empty commands file → empty TSV, exit 0
#   3. single successful worker → one TSV row, dispatcher exits 0
#   4. single failing worker (exit 7) → TSV records exit_code=7, dispatcher exits non-zero
#   5. three parallel successful workers → three TSV rows, dispatcher exits 0
#   6. concurrency cap honored (N=2 with 4 sleep-1 workers → wall clock ~2s, not ~1s or ~4s)
#   7. timeout honored (worker sleeping 5s with timeout=1s → exit code 124, dispatcher exits non-zero)
#   8. WAIT-ALL: one worker fails fast, another succeeds slowly → both rows in TSV
#
# Bash 3.2 compatible per CLAUDE.md (no declare -A, no mapfile, no GNU-only flags).

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$REPO_ROOT/scripts/fanout-dispatch.sh"

PASS=0
FAIL=0

pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; }

fresh_workspace() {
    mktemp -d -t fanout-dispatch-test.XXXXXX
}

# --- Test 1: script exists ---------------------------------------------------
header "Test 1: scripts/fanout-dispatch.sh exists and is executable"
if [ -f "$SCRIPT" ] && [ -x "$SCRIPT" ]; then
    pass "$SCRIPT present and executable"
else
    fail_ "$SCRIPT missing or not executable — Sprint 1.1 implementation required"
    echo
    echo "=== Summary ==="
    echo "  PASS: $PASS"
    echo "  FAIL: $FAIL"
    exit 1
fi

# --- Test 2: empty commands file ---------------------------------------------
header "Test 2: empty commands file → empty TSV, exit 0"
WS=$(fresh_workspace)
: > "$WS/commands.tsv"
rc=0
"$SCRIPT" "$WS/commands.tsv" "$WS/results.tsv" >/dev/null 2>&1 || rc=$?
if [ "$rc" -eq 0 ]; then
    pass "empty commands → exit 0"
else
    fail_ "expected exit 0 on empty input, got $rc"
fi
if [ -f "$WS/results.tsv" ] && [ ! -s "$WS/results.tsv" ]; then
    pass "empty commands → empty TSV"
else
    fail_ "expected empty TSV at $WS/results.tsv"
fi
rm -rf "$WS"

# --- Test 3: single successful worker ----------------------------------------
header "Test 3: single successful worker → 1 TSV row, exit 0"
WS=$(fresh_workspace)
printf 'hello\ttrue\n' > "$WS/commands.tsv"
rc=0
"$SCRIPT" "$WS/commands.tsv" "$WS/results.tsv" >"$WS/stdout" 2>"$WS/stderr" || rc=$?
if [ "$rc" -eq 0 ]; then
    pass "single success → exit 0"
else
    fail_ "expected exit 0, got $rc; stderr: $(cat "$WS/stderr" 2>/dev/null | head -3)"
fi
ROWS=$(wc -l < "$WS/results.tsv" 2>/dev/null | tr -d ' ')
if [ "$ROWS" = "1" ]; then
    pass "TSV has exactly 1 row"
else
    fail_ "expected 1 row, got '$ROWS'"
fi
NAME=$(awk -F'\t' '{print $1}' "$WS/results.tsv" 2>/dev/null)
EXIT=$(awk -F'\t' '{print $2}' "$WS/results.tsv" 2>/dev/null)
if [ "$NAME" = "hello" ] && [ "$EXIT" = "0" ]; then
    pass "row records name=hello exit_code=0"
else
    fail_ "expected hello/0, got '$NAME'/'$EXIT'"
fi
rm -rf "$WS"

# --- Test 4: single failing worker -------------------------------------------
header "Test 4: single failing worker (exit 7) → TSV records exit_code=7"
WS=$(fresh_workspace)
printf 'flop\texit 7\n' > "$WS/commands.tsv"
rc=0
"$SCRIPT" "$WS/commands.tsv" "$WS/results.tsv" >/dev/null 2>&1 || rc=$?
if [ "$rc" -ne 0 ]; then
    pass "any failure → dispatcher exits non-zero (got $rc)"
else
    fail_ "expected non-zero exit on worker failure, got 0"
fi
EXIT=$(awk -F'\t' '$1=="flop"{print $2}' "$WS/results.tsv" 2>/dev/null)
if [ "$EXIT" = "7" ]; then
    pass "TSV records exit_code=7"
else
    fail_ "expected exit_code=7, got '$EXIT'"
fi
rm -rf "$WS"

# --- Test 5: three parallel workers ------------------------------------------
header "Test 5: three parallel successful workers → 3 TSV rows, exit 0"
WS=$(fresh_workspace)
{
    printf 'a\ttrue\n'
    printf 'b\ttrue\n'
    printf 'c\ttrue\n'
} > "$WS/commands.tsv"
rc=0
"$SCRIPT" "$WS/commands.tsv" "$WS/results.tsv" >/dev/null 2>&1 || rc=$?
if [ "$rc" -eq 0 ]; then
    pass "three successes → exit 0"
else
    fail_ "expected exit 0, got $rc"
fi
ROWS=$(wc -l < "$WS/results.tsv" 2>/dev/null | tr -d ' ')
if [ "$ROWS" = "3" ]; then
    pass "TSV has exactly 3 rows"
else
    fail_ "expected 3 rows, got '$ROWS'"
fi
rm -rf "$WS"

# --- Test 6: concurrency cap honored -----------------------------------------
header "Test 6: concurrency cap N=2 with 4 sleep-1 workers → ~2s wall clock"
WS=$(fresh_workspace)
{
    printf 'w1\tsleep 1\n'
    printf 'w2\tsleep 1\n'
    printf 'w3\tsleep 1\n'
    printf 'w4\tsleep 1\n'
} > "$WS/commands.tsv"
START=$(date +%s)
EVOLVE_FANOUT_CONCURRENCY=2 "$SCRIPT" "$WS/commands.tsv" "$WS/results.tsv" >/dev/null 2>&1
END=$(date +%s)
ELAPSED=$((END - START))
# With N=2 and 4 sleep-1 workers, expected ~2s (two batches of 2). Allow 1-3s window.
if [ "$ELAPSED" -ge 2 ] && [ "$ELAPSED" -le 3 ]; then
    pass "wall clock ${ELAPSED}s (expected ~2s for N=2 cap)"
elif [ "$ELAPSED" -lt 2 ]; then
    fail_ "wall clock ${ELAPSED}s too low — concurrency cap not enforced (would have been <1s with no cap)"
else
    fail_ "wall clock ${ELAPSED}s too high — workers may not be parallel within batches"
fi
rm -rf "$WS"

# --- Test 7: timeout honored -------------------------------------------------
header "Test 7: worker sleeping 5s with timeout=1s → exit code 124"
WS=$(fresh_workspace)
printf 'slowpoke\tsleep 5\n' > "$WS/commands.tsv"
rc=0
EVOLVE_FANOUT_TIMEOUT=1 "$SCRIPT" "$WS/commands.tsv" "$WS/results.tsv" >/dev/null 2>&1 || rc=$?
if [ "$rc" -ne 0 ]; then
    pass "timeout → dispatcher exits non-zero"
else
    fail_ "expected non-zero exit on timeout"
fi
EXIT=$(awk -F'\t' '$1=="slowpoke"{print $2}' "$WS/results.tsv" 2>/dev/null)
# `timeout` returns 124 on macOS gtimeout / GNU coreutils. BSD `timeout` from
# coreutils-installed homebrew uses 124 too. Accept 124 or 143 (SIGTERM-killed).
if [ "$EXIT" = "124" ] || [ "$EXIT" = "143" ]; then
    pass "TSV records timeout exit code ($EXIT)"
else
    fail_ "expected exit_code 124 or 143, got '$EXIT'"
fi
rm -rf "$WS"

# --- Test 8: WAIT-ALL semantics ----------------------------------------------
header "Test 8: WAIT-ALL — fast-fail + slow-success both recorded"
WS=$(fresh_workspace)
{
    printf 'fast_fail\texit 3\n'
    printf 'slow_ok\tsleep 1 && true\n'
} > "$WS/commands.tsv"
rc=0
"$SCRIPT" "$WS/commands.tsv" "$WS/results.tsv" >/dev/null 2>&1 || rc=$?
if [ "$rc" -ne 0 ]; then
    pass "any failure → dispatcher exits non-zero"
else
    fail_ "expected non-zero exit"
fi
ROWS=$(wc -l < "$WS/results.tsv" 2>/dev/null | tr -d ' ')
if [ "$ROWS" = "2" ]; then
    pass "both workers recorded in TSV (WAIT-ALL)"
else
    fail_ "expected 2 rows (WAIT-ALL would record both), got '$ROWS'"
fi
FAST_EXIT=$(awk -F'\t' '$1=="fast_fail"{print $2}' "$WS/results.tsv" 2>/dev/null)
SLOW_EXIT=$(awk -F'\t' '$1=="slow_ok"{print $2}' "$WS/results.tsv" 2>/dev/null)
if [ "$FAST_EXIT" = "3" ] && [ "$SLOW_EXIT" = "0" ]; then
    pass "fast_fail=3, slow_ok=0 — failure did not cancel slow worker"
else
    fail_ "expected fast_fail=3, slow_ok=0; got fast_fail='$FAST_EXIT', slow_ok='$SLOW_EXIT'"
fi
rm -rf "$WS"

# --- Summary -----------------------------------------------------------------
echo
echo "=== Summary ==="
echo "  PASS: $PASS"
echo "  FAIL: $FAIL"
[ "$FAIL" -eq 0 ]
