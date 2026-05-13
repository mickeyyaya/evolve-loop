#!/usr/bin/env bash
#
# show-cycle-cost-test.sh — Unit tests for show-cycle-cost.sh.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT="$REPO_ROOT/scripts/observability/show-cycle-cost.sh"

PASS=0; FAIL=0; TESTS_TOTAL=0
pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; TESTS_TOTAL=$((TESTS_TOTAL + 1)); }

cleanup_dirs=()
trap 'for d in "${cleanup_dirs[@]}"; do rm -rf "$d"; done' EXIT

# Helper: build a mock cycle workspace with N stdout logs.
make_cycle() {
    local cycle="$1"
    local d
    d=$(mktemp -d -t show-cost.XXXXXX)
    mkdir -p "$d/.evolve/runs/cycle-$cycle"
    echo "$d"
}

write_phase_log() {
    local cycle_dir="$1" phase="$2" cost="$3" cache_read="$4" cache_write="$5" output="$6"
    cat > "$cycle_dir/$phase-stdout.log" <<EOF
{"type":"result","subtype":"success","total_cost_usd":$cost,"usage":{"input_tokens":4,"cache_read_input_tokens":$cache_read,"cache_creation_input_tokens":$cache_write,"output_tokens":$output}}
EOF
}

# Variant: includes the nested cache_creation.ephemeral_{1h,5m}_input_tokens
# breakdown that real claude -p runs emit. Used to test TTL-bucket telemetry.
write_phase_log_with_ttl() {
    local cycle_dir="$1" phase="$2" cost="$3" cache_read="$4" cache_1h="$5" cache_5m="$6" output="$7"
    local cache_write=$((cache_1h + cache_5m))
    cat > "$cycle_dir/$phase-stdout.log" <<EOF
{"type":"result","subtype":"success","total_cost_usd":$cost,"usage":{"input_tokens":4,"cache_read_input_tokens":$cache_read,"cache_creation_input_tokens":$cache_write,"output_tokens":$output,"cache_creation":{"ephemeral_1h_input_tokens":$cache_1h,"ephemeral_5m_input_tokens":$cache_5m}}}
EOF
}

# === Test 1: missing cycle arg → exit 10 ====================================
header "Test 1: no cycle arg → exit 10"
set +e; out=$(bash "$SCRIPT" 2>&1); rc=$?; set -e
if [ "$rc" = "10" ]; then pass "missing arg → 10"
else fail_ "rc=$rc out=$out"; fi

# === Test 2: non-integer cycle → exit 10 ====================================
header "Test 2: cycle='abc' → exit 10"
set +e; out=$(bash "$SCRIPT" abc 2>&1); rc=$?; set -e
if [ "$rc" = "10" ]; then pass "non-int → 10"
else fail_ "rc=$rc out=$out"; fi

# === Test 3: missing workspace → exit 1 =====================================
header "Test 3: cycle workspace doesn't exist → exit 1"
set +e; out=$(bash "$SCRIPT" 999999 2>&1); rc=$?; set -e
if [ "$rc" = "1" ]; then pass "missing workspace → 1"
else fail_ "rc=$rc out=$out"; fi

# === Test 4: empty workspace (no stdout logs) → exit 1 ======================
header "Test 4: workspace with no *-stdout.log → exit 1"
d=$(make_cycle 999); cleanup_dirs+=("$d")
set +e; out=$(EVOLVE_REPO_ROOT_OVERRIDE="$d" bash "$SCRIPT" 999 2>&1); rc=$?; set -e
# Note: script uses its own REPO_ROOT detection; we need to copy the script in.
mkdir -p "$d/scripts/observability"
cp "$SCRIPT" "$d/scripts/observability/show-cycle-cost.sh"
set +e; out=$(bash "$d/scripts/observability/show-cycle-cost.sh" 999 2>&1); rc=$?; set -e
if [ "$rc" = "1" ]; then pass "empty workspace → 1"
else fail_ "rc=$rc out=$out"; fi

# === Test 5: single phase with cost data → human table prints =================
header "Test 5: 1-phase cycle → human table"
d=$(make_cycle 998); cleanup_dirs+=("$d")
mkdir -p "$d/scripts/observability"
cp "$SCRIPT" "$d/scripts/observability/show-cycle-cost.sh"
write_phase_log "$d/.evolve/runs/cycle-998" "scout" "0.5128" "101097" "39751" "1533"
set +e; out=$(bash "$d/scripts/observability/show-cycle-cost.sh" 998 2>&1); rc=$?; set -e
if [ "$rc" = "0" ] && echo "$out" | grep -q "scout" && echo "$out" | grep -q "0.5128" && echo "$out" | grep -q "TOTAL"; then
    pass "single phase rendered with cost + total row"
else
    fail_ "rc=$rc out=$out"
fi

# === Test 6: multi-phase totals correct ======================================
header "Test 6: 2-phase cycle → totals = sum of phases"
d=$(make_cycle 997); cleanup_dirs+=("$d")
mkdir -p "$d/scripts/observability"
cp "$SCRIPT" "$d/scripts/observability/show-cycle-cost.sh"
write_phase_log "$d/.evolve/runs/cycle-997" "scout"   "0.50" "100" "200" "50"
write_phase_log "$d/.evolve/runs/cycle-997" "auditor" "1.00" "200" "300" "75"
set +e; out=$(bash "$d/scripts/observability/show-cycle-cost.sh" 997 --json 2>&1); rc=$?; set -e
total_cost=$(echo "$out" | jq -r '.total.cost_usd' 2>/dev/null)
total_cr=$(echo "$out" | jq -r '.total.cache_read_input_tokens' 2>/dev/null)
total_cw=$(echo "$out" | jq -r '.total.cache_creation_input_tokens' 2>/dev/null)
total_ot=$(echo "$out" | jq -r '.total.output_tokens' 2>/dev/null)
# Numeric comparison via bc to avoid 1.5 vs 1.50 string mismatch.
cost_match=$(echo "$total_cost == 1.5" | bc -l 2>/dev/null || echo 0)
if [ "$cost_match" = "1" ] && [ "$total_cr" = "300" ] && [ "$total_cw" = "500" ] && [ "$total_ot" = "125" ]; then
    pass "totals: cost=1.5, cr=300, cw=500, ot=125"
else
    fail_ "totals mismatch: cost=$total_cost cr=$total_cr cw=$total_cw ot=$total_ot"
fi

# === Test 7: --json shape valid ==============================================
header "Test 7: --json output is valid JSON with phases array"
d=$(make_cycle 996); cleanup_dirs+=("$d")
mkdir -p "$d/scripts/observability"
cp "$SCRIPT" "$d/scripts/observability/show-cycle-cost.sh"
write_phase_log "$d/.evolve/runs/cycle-996" "scout" "0.50" "100" "200" "50"
out=$(bash "$d/scripts/observability/show-cycle-cost.sh" 996 --json 2>&1)
phase_count=$(echo "$out" | jq -r '.phases | length' 2>/dev/null)
cycle_field=$(echo "$out" | jq -r '.cycle' 2>/dev/null)
if [ "$phase_count" = "1" ] && [ "$cycle_field" = "996" ]; then
    pass "json schema correct"
else
    fail_ "phase_count=$phase_count cycle=$cycle_field"
fi

# === Test 8: log with malformed JSON → graceful (zero values, no crash) ======
header "Test 8: malformed stdout log → graceful zero values"
d=$(make_cycle 995); cleanup_dirs+=("$d")
mkdir -p "$d/scripts/observability"
cp "$SCRIPT" "$d/scripts/observability/show-cycle-cost.sh"
echo "not json at all" > "$d/.evolve/runs/cycle-995/scout-stdout.log"
set +e; out=$(bash "$d/scripts/observability/show-cycle-cost.sh" 995 2>&1); rc=$?; set -e
if [ "$rc" = "0" ] && echo "$out" | grep -q "0.0000"; then
    pass "malformed log → zero values, rc=0"
else
    fail_ "rc=$rc out=$out"
fi

# === Test 9: --json surfaces TTL bucket breakdown per phase + total ==========
header "Test 9: --json includes cache_creation_1h/5m_input_tokens"
d=$(make_cycle 994); cleanup_dirs+=("$d")
mkdir -p "$d/scripts/observability"
cp "$SCRIPT" "$d/scripts/observability/show-cycle-cost.sh"
write_phase_log_with_ttl "$d/.evolve/runs/cycle-994" "scout"   "0.50" "100" "150" "50"  "50"
write_phase_log_with_ttl "$d/.evolve/runs/cycle-994" "auditor" "1.00" "200" "200" "100" "75"
out=$(bash "$d/scripts/observability/show-cycle-cost.sh" 994 --json 2>&1)
scout_1h=$(echo "$out" | jq -r '.phases[] | select(.phase=="scout") | .cache_creation_1h_input_tokens' 2>/dev/null)
scout_5m=$(echo "$out" | jq -r '.phases[] | select(.phase=="scout") | .cache_creation_5m_input_tokens' 2>/dev/null)
total_1h=$(echo "$out" | jq -r '.total.cache_creation_1h_input_tokens' 2>/dev/null)
total_5m=$(echo "$out" | jq -r '.total.cache_creation_5m_input_tokens' 2>/dev/null)
if [ "$scout_1h" = "150" ] && [ "$scout_5m" = "50" ] && [ "$total_1h" = "350" ] && [ "$total_5m" = "150" ]; then
    pass "TTL split: scout 1h=150 5m=50; total 1h=350 5m=150"
else
    fail_ "scout_1h=$scout_1h scout_5m=$scout_5m total_1h=$total_1h total_5m=$total_5m"
fi

# === Test 10: legacy logs without nested cache_creation default to 0 ==========
header "Test 10: backward-compat — legacy logs (no nested cache_creation) → 0"
d=$(make_cycle 993); cleanup_dirs+=("$d")
mkdir -p "$d/scripts/observability"
cp "$SCRIPT" "$d/scripts/observability/show-cycle-cost.sh"
write_phase_log "$d/.evolve/runs/cycle-993" "scout" "0.50" "100" "200" "50"
out=$(bash "$d/scripts/observability/show-cycle-cost.sh" 993 --json 2>&1)
scout_1h=$(echo "$out" | jq -r '.phases[0].cache_creation_1h_input_tokens' 2>/dev/null)
scout_5m=$(echo "$out" | jq -r '.phases[0].cache_creation_5m_input_tokens' 2>/dev/null)
if [ "$scout_1h" = "0" ] && [ "$scout_5m" = "0" ]; then
    pass "legacy log → 1h=0, 5m=0"
else
    fail_ "scout_1h=$scout_1h scout_5m=$scout_5m"
fi

# === Test 11: human-readable footer surfaces TTL split ========================
header "Test 11: human output includes TTL bucket footer"
d=$(make_cycle 992); cleanup_dirs+=("$d")
mkdir -p "$d/scripts/observability"
cp "$SCRIPT" "$d/scripts/observability/show-cycle-cost.sh"
write_phase_log_with_ttl "$d/.evolve/runs/cycle-992" "auditor" "1.00" "500" "850" "0" "100"
out=$(bash "$d/scripts/observability/show-cycle-cost.sh" 992 2>&1)
# Footer must mention both 1h and 5m buckets so a regression (e.g., Anthropic
# flipping the account flag) becomes visible in the cycle summary.
if echo "$out" | grep -qE "1h.*850|850.*1h" && echo "$out" | grep -qE "5m.*0|0.*5m"; then
    pass "footer mentions 1h=850 and 5m=0"
else
    fail_ "footer missing TTL bucket breakdown: $out"
fi

# === Test 12: v9.2.0 — NDJSON stdout.log (stream-json format) is parsed correctly
# When claude.sh uses --output-format=stream-json (the default since v9.2.0),
# stdout.log is multi-line NDJSON: one event per line, with the final `result`
# event carrying usage/cost. Ensure the new grep-based filter extracts the
# correct event regardless of how many other events precede it.
header "Test 12: v9.2.0 — multi-line NDJSON stdout.log → extracts result event"
d=$(make_cycle 990); cleanup_dirs+=("$d")
cat > "$d/.evolve/runs/cycle-990/scout-stdout.log" <<EOF
{"type":"system","subtype":"init","session_id":"s-1","model":"sonnet"}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"begin"}]}}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"tu1","name":"Read","input":{"file_path":"foo"}}]}}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"tu1","content":"ok"}]}}
{"type":"result","subtype":"success","total_cost_usd":0.42,"usage":{"input_tokens":10,"cache_read_input_tokens":1000,"cache_creation_input_tokens":200,"output_tokens":50}}
EOF
set +u; out=$(RUNS_DIR_OVERRIDE="$d/.evolve/runs" bash "$SCRIPT" 990 --json 2>&1); set -u
scout_cost=$(echo "$out" | jq -r '.phases[0].cost_usd // 0' 2>/dev/null || echo MISSING)
if [ "$scout_cost" = "0.42" ]; then
    pass "stream-json NDJSON → cost_usd=0.42 extracted from result event (despite 4 prior non-result lines)"
else
    fail_ "expected cost_usd=0.42; got scout_cost=$scout_cost; out=$out"
fi

# === Test 13: v9.2.0 — final non-result line in NDJSON does NOT confuse parser
# Edge case: a stream where the LAST line is a system or error event (not result).
# The old `tail -1` would grab that and report 0 cost; the new grep MUST find
# the actual result event further back.
header "Test 13: v9.2.0 — result event followed by trailing system event"
d=$(make_cycle 991); cleanup_dirs+=("$d")
cat > "$d/.evolve/runs/cycle-991/scout-stdout.log" <<EOF
{"type":"system","subtype":"init","session_id":"s-2","model":"sonnet"}
{"type":"result","subtype":"success","total_cost_usd":1.75,"usage":{"input_tokens":1,"cache_read_input_tokens":500,"cache_creation_input_tokens":100,"output_tokens":25}}
{"type":"system","subtype":"shutdown"}
EOF
set +u; out=$(RUNS_DIR_OVERRIDE="$d/.evolve/runs" bash "$SCRIPT" 991 --json 2>&1); set -u
scout_cost=$(echo "$out" | jq -r '.phases[0].cost_usd // 0' 2>/dev/null || echo MISSING)
if [ "$scout_cost" = "1.75" ]; then
    pass "result event found despite trailing non-result line — grep filter robust"
else
    fail_ "expected cost_usd=1.75; got scout_cost=$scout_cost; out=$out"
fi


echo
echo "=========================================="
echo "  Total tests: $TESTS_TOTAL"
echo "  Passed:      $PASS"
echo "  Failed:      $FAIL"
echo "=========================================="
[ "$FAIL" = "0" ] && exit 0 || exit 1
