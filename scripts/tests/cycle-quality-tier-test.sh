#!/usr/bin/env bash
# cycle-quality-tier-test.sh — Tests for v8.52.0 cycle-level quality_tier
# composition (GAP-4) and fan-out ledger quality_tier annotation (GAP-3).

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CYCLE_STATE="$REPO_ROOT/scripts/lifecycle/cycle-state.sh"
SUBAGENT_RUN="$REPO_ROOT/scripts/dispatch/subagent-run.sh"

PASS=0
FAIL=0
pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; }

make_state_env() {
    local d
    d=$(mktemp -d -t "cqt.XXXXXX")
    EVOLVE_PROJECT_ROOT="$d" bash "$CYCLE_STATE" init 1 .evolve/runs/cycle-1 >/dev/null 2>&1
    echo "$d"
}

# Test 1: record-quality-tier accepts valid tiers
header "Test 1: record-quality-tier accepts valid tiers"
TMPP=$(make_state_env)
all_ok=1
for t in full hybrid degraded none unknown; do
    if ! EVOLVE_PROJECT_ROOT="$TMPP" bash "$CYCLE_STATE" record-quality-tier "$t" 2>/dev/null; then
        all_ok=0
    fi
done
[ "$all_ok" = "1" ] && pass "all 5 valid tiers accepted" || fail_ "some valid tiers rejected"
rm -rf "$TMPP"

# Test 2: rejects invalid
header "Test 2: rejects invalid tier"
TMPP=$(make_state_env)
set +e
EVOLVE_PROJECT_ROOT="$TMPP" bash "$CYCLE_STATE" record-quality-tier bogus 2>/dev/null
rc=$?
set -e
[ "$rc" != "0" ] && pass "invalid rejected (rc=$rc)" || fail_ "invalid accepted"
rm -rf "$TMPP"

# Test 3: appends
header "Test 3: appends to quality_tiers[] array"
TMPP=$(make_state_env)
EVOLVE_PROJECT_ROOT="$TMPP" bash "$CYCLE_STATE" record-quality-tier full >/dev/null
EVOLVE_PROJECT_ROOT="$TMPP" bash "$CYCLE_STATE" record-quality-tier hybrid >/dev/null
EVOLVE_PROJECT_ROOT="$TMPP" bash "$CYCLE_STATE" record-quality-tier degraded >/dev/null
count=$(jq -r '.quality_tiers | length' "$TMPP/.evolve/cycle-state.json")
[ "$count" = "3" ] && pass "3 entries" || fail_ "count=$count"
rm -rf "$TMPP"

# Test 4: minimum
header "Test 4: compute-cycle-tier returns minimum"
TMPP=$(make_state_env)
EVOLVE_PROJECT_ROOT="$TMPP" bash "$CYCLE_STATE" record-quality-tier full >/dev/null
EVOLVE_PROJECT_ROOT="$TMPP" bash "$CYCLE_STATE" record-quality-tier hybrid >/dev/null
EVOLVE_PROJECT_ROOT="$TMPP" bash "$CYCLE_STATE" record-quality-tier degraded >/dev/null
result=$(EVOLVE_PROJECT_ROOT="$TMPP" bash "$CYCLE_STATE" compute-cycle-tier)
[ "$result" = "degraded" ] && pass "min = degraded" || fail_ "got $result"
rm -rf "$TMPP"

# Test 5: all-full
header "Test 5: all-full → full"
TMPP=$(make_state_env)
for _ in 1 2 3; do EVOLVE_PROJECT_ROOT="$TMPP" bash "$CYCLE_STATE" record-quality-tier full >/dev/null; done
result=$(EVOLVE_PROJECT_ROOT="$TMPP" bash "$CYCLE_STATE" compute-cycle-tier)
[ "$result" = "full" ] && pass "all-full → full" || fail_ "got $result"
rm -rf "$TMPP"

# Test 6: empty → unknown
header "Test 6: no records → unknown"
TMPP=$(make_state_env)
result=$(EVOLVE_PROJECT_ROOT="$TMPP" bash "$CYCLE_STATE" compute-cycle-tier)
[ "$result" = "unknown" ] && pass "empty → unknown" || fail_ "got $result"
rm -rf "$TMPP"

# Test 7: any none wins
header "Test 7: any none → cycle=none"
TMPP=$(make_state_env)
EVOLVE_PROJECT_ROOT="$TMPP" bash "$CYCLE_STATE" record-quality-tier full >/dev/null
EVOLVE_PROJECT_ROOT="$TMPP" bash "$CYCLE_STATE" record-quality-tier none >/dev/null
EVOLVE_PROJECT_ROOT="$TMPP" bash "$CYCLE_STATE" record-quality-tier full >/dev/null
result=$(EVOLVE_PROJECT_ROOT="$TMPP" bash "$CYCLE_STATE" compute-cycle-tier)
[ "$result" = "none" ] && pass "any none → none" || fail_ "got $result"
rm -rf "$TMPP"

# Test 8: subagent-run.sh has fanout_quality_tier resolution
header "Test 8: subagent-run.sh dispatch_parallel resolves fanout_quality_tier"
if grep -q "fanout_quality_tier=" "$SUBAGENT_RUN"; then
    pass "fanout_quality_tier resolved"
else
    fail_ "not wired"
fi

# Test 9: _write_fanout_ledger_entry has 10th arg
header "Test 9: _write_fanout_ledger_entry quality_tier 10th arg"
if grep -q 'local quality_tier="${10:-unknown}"' "$SUBAGENT_RUN"; then
    pass "10th arg present"
else
    fail_ "missing"
fi

# Test 10: agent_fanout JSON has quality_tier
header "Test 10: agent_fanout JSON includes quality_tier"
if grep -q 'quality_tier: \$quality_tier}' "$SUBAGENT_RUN"; then
    pass "quality_tier in JSON template"
else
    fail_ "missing from JSON"
fi

echo
echo "==========================================="
echo "  Total: 10 tests"
echo "  PASS:  $PASS"
echo "  FAIL:  $FAIL"
echo "==========================================="
[ "$FAIL" -eq 0 ] && exit 0 || exit 1
