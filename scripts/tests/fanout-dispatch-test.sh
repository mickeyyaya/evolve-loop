#!/usr/bin/env bash
#
# fanout-dispatch-test.sh — Unit tests for scripts/dispatch/fanout-dispatch.sh.
#
# fanout-dispatch.sh is the parallel-worker dispatcher used by Sprint 1
# of the swarm architecture (see plans/does-the-flow-allow-jaunty-hummingbird.md).
# It runs N worker commands concurrently (bounded by EVOLVE_FANOUT_CONCURRENCY,
# default 2 since v8.55.0 — lowered from 4 to halve peak token-burn rate during
# fan-out so subscription quotas are not exhausted on multi-hour /loop runs),
# enforces a per-worker timeout (EVOLVE_FANOUT_TIMEOUT, default 600s), captures
# each worker's exit code + duration to a TSV file, and uses WAIT-ALL semantics
# (every worker runs to completion or timeout regardless of others' failures).
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
#   9–14. v8.23 cache-prefix + worker-tracking + consensus-cancel coverage
#   15. v8.55.0 — default-cap test: unset EVOLVE_FANOUT_CONCURRENCY → cap=2 (NOT 4)
#   16. v8.55.0 — operator-override test: EVOLVE_FANOUT_CONCURRENCY=4 → all 4 in parallel
#   17. v8.55.0 — cap-1 edge case: EVOLVE_FANOUT_CONCURRENCY=1 → workers serialize
#   18. v8.55.0 Phase E — default per-worker budget cap = $0.20 (auto-injected into worker env)
#   19. v8.55.0 Phase E — operator EVOLVE_MAX_BUDGET_USD override preserved (not clobbered)
#   20. v8.55.0 Phase E — EVOLVE_FANOUT_PER_WORKER_BUDGET_USD overrides default
#   21. v8.55.0 Phase E — malformed budget value falls back to 0.20 (no crash)
#
# Bash 3.2 compatible per CLAUDE.md (no declare -A, no mapfile, no GNU-only flags).

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT="$REPO_ROOT/scripts/dispatch/fanout-dispatch.sh"

PASS=0
FAIL=0

pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; }

fresh_workspace() {
    mktemp -d -t fanout-dispatch-test.XXXXXX
}

# --- Test 1: script exists ---------------------------------------------------
header "Test 1: scripts/dispatch/fanout-dispatch.sh exists and is executable"
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

# --- v8.23.0 tests below (Tasks B, C, D) ------------------------------------

# --- Test 9: Task C — --cache-prefix-file flag accepted, exported to workers
header "Test 9: --cache-prefix-file=PATH flag exposes \$EVOLVE_FANOUT_CACHE_PREFIX_FILE to workers"
WS=$(fresh_workspace)
mkdir -p "$WS"
PREFIX_FILE="$WS/cache-prefix.md"
printf 'shared prefix bytes\n' > "$PREFIX_FILE"
# Worker that prints the env var to its stdout so we can assert.
printf 'check_env\techo PREFIX_PATH=$EVOLVE_FANOUT_CACHE_PREFIX_FILE\n' > "$WS/cmds.tsv"
EVOLVE_FANOUT_TRACK_WORKERS=0 \
    bash "$SCRIPT" --cache-prefix-file "$PREFIX_FILE" "$WS/cmds.tsv" "$WS/results.tsv" >/dev/null 2>&1
RC=$?
WORKER_OUT=$(cat "$WS/check_env.out" 2>/dev/null || echo "")
if [ "$RC" = "0" ] && echo "$WORKER_OUT" | grep -q "^PREFIX_PATH=$PREFIX_FILE$"; then
    pass "worker received EVOLVE_FANOUT_CACHE_PREFIX_FILE env"
else
    fail_ "rc=$RC, worker_out='$WORKER_OUT'"
fi
rm -rf "$WS"

# --- Test 10: Task C — missing --cache-prefix-file path → exit 2 -------------
header "Test 10: --cache-prefix-file pointing to nonexistent file → exit 2"
WS=$(fresh_workspace)
printf 'noop\ttrue\n' > "$WS/cmds.tsv"
set +e
bash "$SCRIPT" --cache-prefix-file "$WS/does-not-exist.md" "$WS/cmds.tsv" "$WS/results.tsv" >/dev/null 2>&1
RC=$?
set -e
if [ "$RC" = "2" ]; then
    pass "missing prefix file → rc=2"
else
    fail_ "expected rc=2, got rc=$RC"
fi
rm -rf "$WS"

# --- Test 11: Task D — worker status callbacks update cycle-state ------------
header "Test 11: EVOLVE_FANOUT_TRACK_WORKERS=1 writes worker status to cycle-state.json"
WS=$(fresh_workspace)
# Provide a stub cycle-state.json + mock cycle-state.sh so we don't depend on
# the real EVOLVE_PROJECT_ROOT.
STATE="$WS/cycle-state.json"
echo '{"cycle_id":1,"phase":"research"}' > "$STATE"
mkdir -p "$WS/scripts/lifecycle"
cat > "$WS/scripts/lifecycle/cycle-state.sh" <<'STUB'
#!/usr/bin/env bash
# Stub: log calls to a sibling .calls file so the test can assert.
echo "$@" >> "$EVOLVE_CYCLE_STATE_FILE.calls"
STUB
chmod +x "$WS/scripts/lifecycle/cycle-state.sh"
printf 'noop\ttrue\n' > "$WS/cmds.tsv"
EVOLVE_PLUGIN_ROOT="$WS" \
EVOLVE_CYCLE_STATE_FILE="$STATE" \
EVOLVE_FANOUT_TRACK_WORKERS=1 \
    bash "$SCRIPT" "$WS/cmds.tsv" "$WS/results.tsv" >/dev/null 2>&1
CALLS=$(cat "$STATE.calls" 2>/dev/null)
if echo "$CALLS" | grep -q "set-worker-status noop running" \
   && echo "$CALLS" | grep -q "set-worker-status noop done 0"; then
    pass "worker status: running → done 0 recorded"
else
    fail_ "expected running + done 0 calls; got: $CALLS"
fi
rm -rf "$WS"

# --- Test 12: Task D — track-workers disabled → no cycle-state writes --------
header "Test 12: EVOLVE_FANOUT_TRACK_WORKERS=0 → no cycle-state.sh calls"
WS=$(fresh_workspace)
mkdir -p "$WS/scripts/lifecycle"
cat > "$WS/scripts/lifecycle/cycle-state.sh" <<'STUB'
#!/usr/bin/env bash
echo "$@" >> "$WS_CALLS_LOG"
STUB
chmod +x "$WS/scripts/lifecycle/cycle-state.sh"
printf 'noop\ttrue\n' > "$WS/cmds.tsv"
EVOLVE_PLUGIN_ROOT="$WS" \
EVOLVE_FANOUT_TRACK_WORKERS=0 \
WS_CALLS_LOG="$WS/calls.log" \
    bash "$SCRIPT" "$WS/cmds.tsv" "$WS/results.tsv" >/dev/null 2>&1
if [ ! -s "$WS/calls.log" ]; then
    pass "track-workers=0 → no cycle-state.sh calls"
else
    fail_ "expected no calls; got: $(cat $WS/calls.log)"
fi
rm -rf "$WS"

# --- Test 13: Task B — consensus-cancel SIGTERMs slow workers when 2 fail ----
header "Test 13: EVOLVE_FANOUT_CANCEL_ON_CONSENSUS=1 cancels remaining when K=2 FAIL"
WS=$(fresh_workspace)
# 4 workers: 2 emit Verdict: FAIL quickly; 2 sleep long. Without consensus,
# wall-time = max(slow workers) ~5s. With consensus, wall-time ~1-2s.
{
    printf 'fast_fail_a\tprintf "Verdict: FAIL\\n" >&1; exit 1\n'
    printf 'fast_fail_b\tprintf "Verdict: FAIL\\n" >&1; exit 1\n'
    printf 'slow_a\tsleep 5; printf "Verdict: PASS\\n" >&1; exit 0\n'
    printf 'slow_b\tsleep 5; printf "Verdict: PASS\\n" >&1; exit 0\n'
} > "$WS/cmds.tsv"
START=$(date +%s)
EVOLVE_FANOUT_TRACK_WORKERS=0 \
EVOLVE_FANOUT_CANCEL_ON_CONSENSUS=1 \
EVOLVE_FANOUT_CONSENSUS_K=2 \
EVOLVE_FANOUT_CONSENSUS_POLL_S=1 \
    bash "$SCRIPT" "$WS/cmds.tsv" "$WS/results.tsv" >/dev/null 2>&1 || true
END=$(date +%s)
ELAPSED=$((END - START))
if [ "$ELAPSED" -le 4 ]; then
    pass "consensus reached → cancelled slow workers (elapsed=${ELAPSED}s ≤ 4s)"
else
    fail_ "expected ≤4s, got ${ELAPSED}s — consensus did not cancel"
fi
rm -rf "$WS"

# --- Test 14: Task B — consensus disabled → WAIT-ALL still applies -----------
header "Test 14: EVOLVE_FANOUT_CANCEL_ON_CONSENSUS=0 (default) → all workers run"
WS=$(fresh_workspace)
{
    printf 'fast_fail\tprintf "Verdict: FAIL\\n" >&1; exit 1\n'
    printf 'slow_ok\tsleep 2; exit 0\n'
} > "$WS/cmds.tsv"
START=$(date +%s)
EVOLVE_FANOUT_TRACK_WORKERS=0 \
    bash "$SCRIPT" "$WS/cmds.tsv" "$WS/results.tsv" >/dev/null 2>&1 || true
END=$(date +%s)
ELAPSED=$((END - START))
ROWS=$(wc -l < "$WS/results.tsv" 2>/dev/null | tr -d ' ')
if [ "$ELAPSED" -ge 2 ] && [ "$ROWS" = "2" ]; then
    pass "WAIT-ALL preserved (elapsed=${ELAPSED}s ≥ 2s, both workers in TSV)"
else
    fail_ "expected ≥2s elapsed and 2 rows; got elapsed=${ELAPSED}s rows=$ROWS"
fi
rm -rf "$WS"

# --- Test 15 (v8.55.0): default-cap — unset env → 2 concurrent workers -------
# Token-burn-rate guardrail. Subscription users on continuous /loop runs cannot
# tolerate 4-concurrent fan-out; default 2 halves peak burn at the cost of
# longer wall time. Unsetting the env var must resolve to 2, not 4.
header "Test 15 (v8.55.0): unset EVOLVE_FANOUT_CONCURRENCY → cap=2 (4 sleep-1 workers → ~2s)"
WS=$(fresh_workspace)
{
    printf 'd1\tsleep 1\n'
    printf 'd2\tsleep 1\n'
    printf 'd3\tsleep 1\n'
    printf 'd4\tsleep 1\n'
} > "$WS/commands.tsv"
START=$(date +%s)
unset EVOLVE_FANOUT_CONCURRENCY
"$SCRIPT" "$WS/commands.tsv" "$WS/results.tsv" >/dev/null 2>&1
END=$(date +%s)
ELAPSED=$((END - START))
# Expected: ~2s (two batches of 2). Reject < 2s (would mean cap > 2 in effect).
# Allow up to 4s for slow CI; reject ≥ 4s (would mean cap=1).
if [ "$ELAPSED" -ge 2 ] && [ "$ELAPSED" -lt 4 ]; then
    pass "default cap honored: ${ELAPSED}s wall time (expected ~2s for cap=2)"
elif [ "$ELAPSED" -lt 2 ]; then
    fail_ "wall time ${ELAPSED}s — default cap appears to be > 2 (token-burn risk!)"
else
    fail_ "wall time ${ELAPSED}s — default cap appears to be 1 or workers serialized"
fi
rm -rf "$WS"

# --- Test 16 (v8.55.0): operator override path — =4 → all parallel -----------
# Confirms that operators on API plans (no quota concern) can still bump the
# cap back to 4 with the existing env var. Mechanism is unchanged; only the
# default value moved.
header "Test 16 (v8.55.0): EVOLVE_FANOUT_CONCURRENCY=4 → all 4 parallel (~1s)"
WS=$(fresh_workspace)
{
    printf 'o1\tsleep 1\n'
    printf 'o2\tsleep 1\n'
    printf 'o3\tsleep 1\n'
    printf 'o4\tsleep 1\n'
} > "$WS/commands.tsv"
START=$(date +%s)
EVOLVE_FANOUT_CONCURRENCY=4 "$SCRIPT" "$WS/commands.tsv" "$WS/results.tsv" >/dev/null 2>&1
END=$(date +%s)
ELAPSED=$((END - START))
# Expected: ~1s (all 4 in single batch). Reject ≥ 2s (would mean override ignored).
if [ "$ELAPSED" -lt 2 ]; then
    pass "override honored: ${ELAPSED}s wall time (expected ~1s for cap=4)"
else
    fail_ "wall time ${ELAPSED}s — override path appears broken (expected <2s for cap=4)"
fi
rm -rf "$WS"

# --- Test 17 (v8.55.0): cap-1 edge case — full serialization ----------------
# Sanity check: setting cap=1 must serialize all workers (degenerate case where
# fan-out reduces to sequential). 4 sleep-1 workers → ~4s wall time.
header "Test 17 (v8.55.0): EVOLVE_FANOUT_CONCURRENCY=1 → workers serialize (~4s)"
WS=$(fresh_workspace)
{
    printf 's1\tsleep 1\n'
    printf 's2\tsleep 1\n'
    printf 's3\tsleep 1\n'
    printf 's4\tsleep 1\n'
} > "$WS/commands.tsv"
START=$(date +%s)
EVOLVE_FANOUT_CONCURRENCY=1 "$SCRIPT" "$WS/commands.tsv" "$WS/results.tsv" >/dev/null 2>&1
END=$(date +%s)
ELAPSED=$((END - START))
# Expected: ~4s (full serial). Allow 4-6s window; reject < 4s (would mean cap > 1).
if [ "$ELAPSED" -ge 4 ] && [ "$ELAPSED" -le 6 ]; then
    pass "cap=1 serializes: ${ELAPSED}s wall time (expected ~4s)"
else
    fail_ "wall time ${ELAPSED}s — cap=1 should serialize 4 workers to ~4s"
fi
rm -rf "$WS"

# --- Test 18 (v8.55.0 Phase E): default per-worker budget = $0.20 -----------
# When neither EVOLVE_MAX_BUDGET_USD nor EVOLVE_FANOUT_PER_WORKER_BUDGET_USD is
# set, fan-out workers must see EVOLVE_MAX_BUDGET_USD=0.20 in their env.
# Total fan-out spend is capped at concurrency × cap × batches.
header "Test 18 (v8.55.0): default per-worker budget exported to worker env"
WS=$(fresh_workspace)
# Worker prints the env var so we can inspect.
printf 'budget_check\techo MAX_BUDGET_USD=$EVOLVE_MAX_BUDGET_USD\n' > "$WS/cmds.tsv"
unset EVOLVE_MAX_BUDGET_USD
unset EVOLVE_FANOUT_PER_WORKER_BUDGET_USD
EVOLVE_FANOUT_TRACK_WORKERS=0 \
    "$SCRIPT" "$WS/cmds.tsv" "$WS/results.tsv" >/dev/null 2>&1
WORKER_OUT=$(cat "$WS/budget_check.out" 2>/dev/null)
if echo "$WORKER_OUT" | grep -q "^MAX_BUDGET_USD=0.20$"; then
    pass "default per-worker budget = 0.20"
else
    fail_ "expected MAX_BUDGET_USD=0.20; got: '$WORKER_OUT'"
fi
rm -rf "$WS"

# --- Test 19 (v8.55.0 Phase E): operator override preserved -----------------
# If operator already set EVOLVE_MAX_BUDGET_USD externally (e.g., via release
# pipeline or per-cycle override), fan-out must NOT clobber it.
header "Test 19 (v8.55.0): EVOLVE_MAX_BUDGET_USD operator override preserved"
WS=$(fresh_workspace)
printf 'budget_check\techo MAX_BUDGET_USD=$EVOLVE_MAX_BUDGET_USD\n' > "$WS/cmds.tsv"
EVOLVE_FANOUT_TRACK_WORKERS=0 \
EVOLVE_MAX_BUDGET_USD=5.00 \
    "$SCRIPT" "$WS/cmds.tsv" "$WS/results.tsv" >/dev/null 2>&1
WORKER_OUT=$(cat "$WS/budget_check.out" 2>/dev/null)
if echo "$WORKER_OUT" | grep -q "^MAX_BUDGET_USD=5.00$"; then
    pass "operator override preserved: 5.00 (NOT clobbered to 0.20)"
else
    fail_ "expected operator value 5.00 preserved; got: '$WORKER_OUT'"
fi
rm -rf "$WS"

# --- Test 20 (v8.55.0 Phase E): fan-out-specific override --------------------
# EVOLVE_FANOUT_PER_WORKER_BUDGET_USD lets operators tune fan-out separately
# from per-cycle budget. Set 0.05 → workers see 0.05.
header "Test 20 (v8.55.0): EVOLVE_FANOUT_PER_WORKER_BUDGET_USD=0.05 → 0.05"
WS=$(fresh_workspace)
printf 'budget_check\techo MAX_BUDGET_USD=$EVOLVE_MAX_BUDGET_USD\n' > "$WS/cmds.tsv"
unset EVOLVE_MAX_BUDGET_USD
EVOLVE_FANOUT_TRACK_WORKERS=0 \
EVOLVE_FANOUT_PER_WORKER_BUDGET_USD=0.05 \
    "$SCRIPT" "$WS/cmds.tsv" "$WS/results.tsv" >/dev/null 2>&1
WORKER_OUT=$(cat "$WS/budget_check.out" 2>/dev/null)
if echo "$WORKER_OUT" | grep -q "^MAX_BUDGET_USD=0.05$"; then
    pass "fan-out-specific override honored: 0.05"
else
    fail_ "expected 0.05; got: '$WORKER_OUT'"
fi
rm -rf "$WS"

# --- Test 21 (v8.55.0 Phase E): malformed value falls back to default --------
# Defensive: bad operator input shouldn't crash; falls back to 0.20.
header "Test 21 (v8.55.0): malformed EVOLVE_FANOUT_PER_WORKER_BUDGET_USD → 0.20 fallback"
WS=$(fresh_workspace)
printf 'budget_check\techo MAX_BUDGET_USD=$EVOLVE_MAX_BUDGET_USD\n' > "$WS/cmds.tsv"
unset EVOLVE_MAX_BUDGET_USD
EVOLVE_FANOUT_TRACK_WORKERS=0 \
EVOLVE_FANOUT_PER_WORKER_BUDGET_USD="foo-bar" \
    "$SCRIPT" "$WS/cmds.tsv" "$WS/results.tsv" >/dev/null 2>&1
RC=$?
WORKER_OUT=$(cat "$WS/budget_check.out" 2>/dev/null)
if [ "$RC" -eq 0 ] && echo "$WORKER_OUT" | grep -q "^MAX_BUDGET_USD=0.20$"; then
    pass "malformed → fallback to 0.20 (no crash; rc=$RC)"
else
    fail_ "expected rc=0 + 0.20 fallback; got rc=$RC, out='$WORKER_OUT'"
fi
rm -rf "$WS"

# --- Summary -----------------------------------------------------------------
echo
echo "=== Summary ==="
echo "  PASS: $PASS"
echo "  FAIL: $FAIL"
[ "$FAIL" -eq 0 ]
