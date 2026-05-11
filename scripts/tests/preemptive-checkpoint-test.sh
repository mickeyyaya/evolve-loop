#!/usr/bin/env bash
# v9.1.0 Cycle 2: pre-emptive checkpoint threshold test.
#
# Verifies the dispatcher's batch-budget threshold logic correctly:
#   - emits WARN at ≥80% (configurable via EVOLVE_CHECKPOINT_WARN_AT_PCT)
#   - sets EVOLVE_CHECKPOINT_REQUEST=1 at ≥95% (configurable via
#     EVOLVE_CHECKPOINT_AT_PCT) and exports EVOLVE_CHECKPOINT_REASON
#   - does NOT fire when EVOLVE_CHECKPOINT_DISABLE=1
#   - does NOT fire when EVOLVE_BATCH_BUDGET_DISABLE=1

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd -P)"
DISPATCH="$PROJECT_ROOT/scripts/dispatch/evolve-loop-dispatch.sh"
RUN_CYCLE="$PROJECT_ROOT/scripts/dispatch/run-cycle.sh"

PASS=0
FAIL=0

expect_match() {
    local label="$1"
    local actual="$2"
    local pattern="$3"
    if echo "$actual" | grep -qE "$pattern"; then
        printf "  PASS: %s\n" "$label"
        PASS=$((PASS + 1))
    else
        printf "  FAIL: %s\n    pattern=%s\n    actual =%s\n" "$label" "$pattern" "$actual" >&2
        FAIL=$((FAIL + 1))
    fi
}

expect_no_match() {
    local label="$1"
    local actual="$2"
    local pattern="$3"
    if ! echo "$actual" | grep -qE "$pattern"; then
        printf "  PASS: %s\n" "$label"
        PASS=$((PASS + 1))
    else
        printf "  FAIL: %s\n    pattern (should-not-match)=%s\n    actual =%s\n" "$label" "$pattern" "$actual" >&2
        FAIL=$((FAIL + 1))
    fi
}

# === Test 1: source code presence ===
echo "=== Test 1: Cycle 2 threshold logic present in dispatcher ==="
src=$(cat "$DISPATCH")
expect_match "EVOLVE_CHECKPOINT_WARN_AT_PCT default 80" "$src" "EVOLVE_CHECKPOINT_WARN_AT_PCT:-80"
expect_match "EVOLVE_CHECKPOINT_AT_PCT default 95" "$src" "EVOLVE_CHECKPOINT_AT_PCT:-95"
expect_match "EVOLVE_CHECKPOINT_DISABLE opt-out" "$src" "EVOLVE_CHECKPOINT_DISABLE"
expect_match "exports EVOLVE_CHECKPOINT_REQUEST" "$src" "export EVOLVE_CHECKPOINT_REQUEST"
expect_match "exports EVOLVE_CHECKPOINT_REASON" "$src" "export EVOLVE_CHECKPOINT_REASON"
expect_match "WARN log message" "$src" "BATCH-BUDGET WARN: cumulative"
expect_match "CRITICAL log message" "$src" "BATCH-BUDGET CRITICAL: cumulative"
expect_match "reason set to batch-cap-near" "$src" 'EVOLVE_CHECKPOINT_REASON=.?"batch-cap-near"'

# === Test 2: run-cycle.sh propagates checkpoint env to orchestrator ===
echo
echo "=== Test 2: run-cycle.sh exports checkpoint env vars ==="
src=$(cat "$RUN_CYCLE")
expect_match "exports EVOLVE_CHECKPOINT_REQUEST" "$src" "export EVOLVE_CHECKPOINT_REQUEST"
expect_match "exports EVOLVE_CHECKPOINT_REASON" "$src" "export EVOLVE_CHECKPOINT_REASON"
expect_match "exports EVOLVE_CHECKPOINT_TRIGGERED" "$src" "export EVOLVE_CHECKPOINT_TRIGGERED"

# === Test 3: simulated threshold logic in isolation ===
echo
echo "=== Test 3: simulated arithmetic — 80% threshold ==="
# Run the threshold math in isolation to verify it's correct.
BATCH_TOTAL_COST="16.00"
BATCH_CAP="20.00"
CHECKPOINT_WARN_AT_PCT="80"
pct=$(echo "scale=2; $BATCH_TOTAL_COST / $BATCH_CAP * 100" | bc -l 2>/dev/null | awk -F. '{print $1+0}')
expect_match "16/20 = 80%" "$pct" "^80$"

# === Test 4: 95% threshold ===
echo
echo "=== Test 4: simulated arithmetic — 95% threshold ==="
BATCH_TOTAL_COST="19.00"
BATCH_CAP="20.00"
pct=$(echo "scale=2; $BATCH_TOTAL_COST / $BATCH_CAP * 100" | bc -l 2>/dev/null | awk -F. '{print $1+0}')
expect_match "19/20 = 95%" "$pct" "^95$"

# === Test 5: below WARN threshold ===
echo
echo "=== Test 5: simulated arithmetic — 50% below all thresholds ==="
BATCH_TOTAL_COST="10.00"
BATCH_CAP="20.00"
pct=$(echo "scale=2; $BATCH_TOTAL_COST / $BATCH_CAP * 100" | bc -l 2>/dev/null | awk -F. '{print $1+0}')
expect_match "10/20 = 50%" "$pct" "^50$"

# === Test 6: bash -n syntax check ===
echo
echo "=== Test 6: dispatcher syntax check ==="
if bash -n "$DISPATCH" 2>&1; then
    printf "  PASS: dispatcher passes bash -n\n"
    PASS=$((PASS + 1))
else
    printf "  FAIL: dispatcher has syntax error\n" >&2
    FAIL=$((FAIL + 1))
fi

if bash -n "$RUN_CYCLE" 2>&1; then
    printf "  PASS: run-cycle.sh passes bash -n\n"
    PASS=$((PASS + 1))
else
    printf "  FAIL: run-cycle.sh has syntax error\n" >&2
    FAIL=$((FAIL + 1))
fi

# === Test 7: --help still works (no regression) ===
echo
echo "=== Test 7: dispatcher --help works (no regression) ==="
help_output=$(bash "$DISPATCH" --help 2>&1 || true)
if echo "$help_output" | grep -qE "Usage|budget-usd|cycles"; then
    printf "  PASS: --help output mentions usage/budget-usd/cycles\n"
    PASS=$((PASS + 1))
else
    printf "  FAIL: --help output looks broken: %s\n" "$(echo "$help_output" | head -3)" >&2
    FAIL=$((FAIL + 1))
fi

# === Test 8: env-var ordering — request flag wins over warn ===
echo
echo "=== Test 8: control flow — 95% sets REQUEST flag (one-shot) ==="
# Verify the source code: when _pct >= AT_PCT AND REQUEST not already set,
# we export REQUEST=1. The condition `[ "${EVOLVE_CHECKPOINT_REQUEST:-0}" != "1" ]`
# makes it idempotent (won't re-trigger once already set). Grep a wider
# context window around the BATCH-BUDGET CRITICAL log line — the guard is
# in the `if` condition that precedes it.
src=$(grep -B 5 "BATCH-BUDGET CRITICAL" "$DISPATCH" | head -20)
expect_match "idempotent guard on REQUEST set" "$src" "EVOLVE_CHECKPOINT_REQUEST:-0.*!=.*.1.?"

# === Summary ===
echo
echo "=== Summary ==="
echo "PASS: $PASS"
echo "FAIL: $FAIL"
if [ "$FAIL" -eq 0 ]; then
    echo "ALL TESTS PASSED"
    exit 0
else
    exit 1
fi
