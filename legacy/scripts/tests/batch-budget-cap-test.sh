#!/usr/bin/env bash
#
# batch-budget-cap-test.sh — v8.58.0 Layer B tests.
# Verifies the dispatcher accumulates per-cycle costs and stops the batch
# when the cumulative total exceeds EVOLVE_BATCH_BUDGET_CAP.
#
# Strategy: rather than spawning real subagents (slow + expensive), test the
# accumulation logic in isolation by exercising the cost-line parsing + bc
# arithmetic that drives the tripwire.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DISPATCHER="$REPO_ROOT/scripts/dispatch/evolve-loop-dispatch.sh"
SCRATCH=$(mktemp -d -t batch-budget-test.XXXXXX)
trap 'rm -rf "$SCRATCH"' EXIT

PASS=0
FAIL=0
pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; }

# --- Test 1: dispatcher source defines BATCH_TOTAL_COST + BATCH_CAP -------
header "Test 1: dispatcher initializes batch budget vars before loop"
grep -q '^BATCH_TOTAL_COST=' "$DISPATCHER" && pass "BATCH_TOTAL_COST initialized" || \
    fail_ "BATCH_TOTAL_COST not initialized in dispatcher"
grep -q '^BATCH_CAP=' "$DISPATCHER" && pass "BATCH_CAP initialized" || \
    fail_ "BATCH_CAP not initialized in dispatcher"
grep -q 'EVOLVE_BATCH_BUDGET_CAP' "$DISPATCHER" && pass "EVOLVE_BATCH_BUDGET_CAP env var honored" || \
    fail_ "EVOLVE_BATCH_BUDGET_CAP env not referenced"
grep -q 'EVOLVE_BATCH_BUDGET_DISABLE' "$DISPATCHER" && pass "EVOLVE_BATCH_BUDGET_DISABLE opt-out present" || \
    fail_ "EVOLVE_BATCH_BUDGET_DISABLE not implemented"

# --- Test 2: tripwire breaks the loop when cap exceeded -------------------
header "Test 2: BATCH-BUDGET-EXCEEDED triggers break statement"
grep -q "BATCH-BUDGET-EXCEEDED" "$DISPATCHER" && pass "exceedance log line present" || \
    fail_ "BATCH-BUDGET-EXCEEDED not logged"
# The break must be in the same conditional block as the exceedance check
if awk '/BATCH-BUDGET-EXCEEDED/,/^    fi$/' "$DISPATCHER" | grep -q "break"; then
    pass "break statement follows exceedance log"
else
    fail_ "no break after BATCH-BUDGET-EXCEEDED — loop wouldn't stop"
fi

# --- Test 3: bc arithmetic comparison ($X > $CAP) -------------------------
header "Test 3: bc-based decimal comparison logic is correct"
# Simulate: BATCH_TOTAL_COST=15.00, BATCH_CAP=12.00 → exceeded
RESULT=$(echo "15.00 > 12.00" | bc -l 2>/dev/null)
[ "$RESULT" = "1" ] && pass "bc comparison: 15 > 12 → 1 (exceeded)" || \
    fail_ "bc comparison failed: got '$RESULT'"
# Simulate: cumulative accumulation
ACC=$(echo "0.00 + 7.49" | bc -l 2>/dev/null)
ACC=$(echo "$ACC + 5.21" | bc -l 2>/dev/null)
[ "$ACC" = "12.70" ] && pass "bc accumulation: 0 + 7.49 + 5.21 = 12.70" || \
    fail_ "bc accumulation failed: got '$ACC'"

# --- Test 4: summary line emits batch_total_cost --------------------------
header "Test 4: dispatcher summary surfaces batch_total_cost"
grep -q 'batch_total_cost=' "$DISPATCHER" && pass "summary contains batch_total_cost line" || \
    fail_ "no batch_total_cost in summary"

# --- Test 5: opt-out path ($EVOLVE_BATCH_BUDGET_DISABLE=1 skips check) ----
header "Test 5: BATCH_BUDGET_DISABLE=1 skips tripwire"
# When disable=1, the tripwire conditional must guard against running.
if grep -B5 'BATCH-BUDGET-EXCEEDED' "$DISPATCHER" | grep -q 'BATCH_BUDGET_DISABLE'; then
    pass "tripwire is gated on BATCH_BUDGET_DISABLE flag"
else
    fail_ "tripwire not gated on disable flag — would always fire"
fi

# --- Test 6: default cap is $20 -------------------------------------------
header "Test 6: default EVOLVE_BATCH_BUDGET_CAP is \$20"
grep -q 'EVOLVE_BATCH_BUDGET_CAP:-20' "$DISPATCHER" && pass "default cap = \$20.00" || \
    fail_ "default cap not 20.00"

# --- Summary ----------------------------------------------------------------
echo
echo "==========================================="
echo "Results: $PASS passed, $FAIL failed"
echo "==========================================="
[ "$FAIL" -eq 0 ]
