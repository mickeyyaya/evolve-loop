#!/usr/bin/env bash
#
# phase-observer-test.sh — Integration tests for phase-observer.sh.
#
# Six fixture scenarios: normal, stuck, infinite_loop, error_spike,
# throttled, phase_end. Each writes a synthetic stdout.log, runs the
# observer with short EOF_GRACE, and asserts events.ndjson + report.json
# contain the expected severities and metric types.
#
# Cost-anomaly is unit-tested at the rule level (no baseline fixture in v1);
# its end-to-end test is deferred until rollup-cycle-metrics provides
# per-phase baselines.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
PLUGIN_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd -P)"
OBSERVER="$PLUGIN_ROOT/scripts/dispatch/phase-observer.sh"
[ -f "$OBSERVER" ] || { echo "FATAL: $OBSERVER not found" >&2; exit 1; }

PASS=0; FAIL=0; TOTAL=0
pass() { PASS=$((PASS+1)); TOTAL=$((TOTAL+1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL+1)); TOTAL=$((TOTAL+1)); echo "  FAIL: $1"; }
header() { echo; echo "=== $* ==="; }

# Run observer against a workspace with short EOF_GRACE for fast tests.
# Args: $1=workspace $2=phase $3=agent [extra env-var-key=val]...
run_observer() {
    local ws=$1 phase=$2 agent=$3; shift 3
    local extra_env=""
    for kv in "$@"; do
        extra_env="$extra_env $kv"
    done
    eval "EVOLVE_PLUGIN_ROOT=$PLUGIN_ROOT \\
        EVOLVE_OBSERVER_POLL_S=1 \\
        EVOLVE_OBSERVER_EOF_GRACE_S=3 \\
        EVOLVE_OBSERVER_HEARTBEAT_EVERY=99 \\
        $extra_env \\
        bash '$OBSERVER' '$ws' 0 99 '$phase' '$agent' /dev/null >/dev/null 2>&1"
}

# ───────────────────────────────────────────────────────────────────────────
# TEST 1 — normal phase: typical events, no rules fire
header "TEST 1 — normal phase: NORMAL verdict, zero incidents"
WS=$(mktemp -d -t obs-test.XXXXXX)
cat > "$WS/scout-stdout.log" <<'F'
{"type":"system","subtype":"init","session_id":"s-1","model":"sonnet"}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"begin"}]}}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"t1","name":"Read","input":{"file_path":"a"}}]}}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"ok"}]}}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"t2","name":"Bash","input":{"command":"ls"}}]}}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t2","content":"file1\nfile2"}]}}
{"type":"result","subtype":"success","total_cost_usd":0.10,"num_turns":3,"usage":{"input_tokens":50,"cache_read_input_tokens":2000,"cache_creation_input_tokens":100,"output_tokens":25}}
F
run_observer "$WS" scout scout EVOLVE_OBSERVER_STALL_S=999

REPORT="$WS/scout-observer-report.json"
EVENTS="$WS/scout-observer-events.ndjson"
[ -f "$REPORT" ] && pass "report file created" || fail "report missing"
[ -f "$EVENTS" ] && pass "events file created" || fail "events missing"
verdict=$(jq -r '.summary.verdict' "$REPORT" 2>/dev/null)
[ "$verdict" = "NORMAL" ] && pass "verdict=NORMAL" || fail "verdict expected NORMAL got $verdict"
incidents=$(jq -r '.incidents | length' "$REPORT" 2>/dev/null)
[ "$incidents" = "0" ] && pass "0 incidents" || fail "expected 0 incidents got $incidents"
phase_end=$(grep -c '"observation.phase_end"' "$EVENTS" 2>/dev/null || echo 0)
[ "$phase_end" -ge "1" ] && pass "phase_end observation emitted" || fail "no phase_end"
rm -rf "$WS"

# ───────────────────────────────────────────────────────────────────────────
# TEST 2 — stuck phase: no events written; stall threshold short enough to fire
header "TEST 2 — stuck phase: INCIDENT verdict, stuck metric"
WS=$(mktemp -d -t obs-test.XXXXXX)
# Empty stdout.log → observer sees no events. STALL_S=2 will fire after 2s idle.
: > "$WS/scout-stdout.log"
# Write one event so EVENT_COUNT > 0 (rules don't run on totally-empty streams
# in the most-cautious branch); then observer sits idle until STALL fires.
echo '{"type":"system","subtype":"init","session_id":"s-2","model":"sonnet"}' > "$WS/scout-stdout.log"
# Use a longer EOF_GRACE so the observer has time to fire the stall rule
# before auto-shutdown.
EVOLVE_PLUGIN_ROOT=$PLUGIN_ROOT \
    EVOLVE_OBSERVER_POLL_S=1 \
    EVOLVE_OBSERVER_EOF_GRACE_S=8 \
    EVOLVE_OBSERVER_HEARTBEAT_EVERY=99 \
    EVOLVE_OBSERVER_STALL_S=3 \
    bash "$OBSERVER" "$WS" 0 99 scout scout /dev/null >/dev/null 2>&1

REPORT="$WS/scout-observer-report.json"
EVENTS="$WS/scout-observer-events.ndjson"
verdict=$(jq -r '.summary.verdict' "$REPORT" 2>/dev/null)
[ "$verdict" = "INCIDENT" ] && pass "verdict=INCIDENT" || fail "verdict expected INCIDENT got $verdict"
stuck_count=$(grep -c '"metric_type":"stuck"' "$EVENTS" 2>/dev/null || echo 0)
[ "$stuck_count" -ge "1" ] && pass "stuck observation emitted (count=$stuck_count)" || fail "no stuck observation"
sev=$(grep '"metric_type":"stuck"' "$EVENTS" | head -1 | jq -r '.severity')
[ "$sev" = "INCIDENT" ] && pass "stuck severity=INCIDENT" || fail "stuck severity expected INCIDENT got $sev"
rm -rf "$WS"

# ───────────────────────────────────────────────────────────────────────────
# TEST 3 — infinite_loop: same Read of same file 7 times
header "TEST 3 — infinite_loop: INCIDENT verdict, loop metric"
WS=$(mktemp -d -t obs-test.XXXXXX)
# Build a fixture with 7 identical tool_use(Read foo) events.
: > "$WS/scout-stdout.log"
echo '{"type":"system","subtype":"init","session_id":"s-3","model":"sonnet"}' >> "$WS/scout-stdout.log"
for i in 1 2 3 4 5 6 7; do
    echo '{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"t'"$i"'","name":"Read","input":{"file_path":"foo"}}]}}' >> "$WS/scout-stdout.log"
    echo '{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t'"$i"'","content":"x"}]}}' >> "$WS/scout-stdout.log"
done
echo '{"type":"result","subtype":"success","total_cost_usd":0.05,"num_turns":7,"usage":{}}' >> "$WS/scout-stdout.log"
run_observer "$WS" scout scout EVOLVE_OBSERVER_STALL_S=999 EVOLVE_OBSERVER_LOOP_N=6 EVOLVE_OBSERVER_LOOP_WINDOW_S=300

REPORT="$WS/scout-observer-report.json"
EVENTS="$WS/scout-observer-events.ndjson"
verdict=$(jq -r '.summary.verdict' "$REPORT" 2>/dev/null)
[ "$verdict" = "INCIDENT" ] && pass "verdict=INCIDENT" || fail "verdict expected INCIDENT got $verdict"
loop_count=$(grep -c '"metric_type":"infinite_loop"' "$EVENTS" 2>/dev/null || echo 0)
[ "$loop_count" -ge "1" ] && pass "infinite_loop observation emitted (count=$loop_count)" || fail "no infinite_loop observation"
sev=$(grep '"metric_type":"infinite_loop"' "$EVENTS" | head -1 | jq -r '.severity')
[ "$sev" = "INCIDENT" ] && pass "loop severity=INCIDENT" || fail "loop severity expected INCIDENT got $sev"
tool=$(grep '"metric_type":"infinite_loop"' "$EVENTS" | head -1 | jq -r '.data.evidence.tool_name')
[ "$tool" = "Read" ] && pass "evidence.tool_name=Read" || fail "tool_name expected Read got $tool"
rm -rf "$WS"

# ───────────────────────────────────────────────────────────────────────────
# TEST 4 — error_spike: 4/10 errors triggers WARN
header "TEST 4 — error_spike: DEGRADED verdict, error_spike metric"
WS=$(mktemp -d -t obs-test.XXXXXX)
: > "$WS/scout-stdout.log"
echo '{"type":"system","subtype":"init","session_id":"s-4","model":"sonnet"}' >> "$WS/scout-stdout.log"
for i in 1 2 3 4 5 6 7 8 9 10; do
    is_err="false"
    [ "$i" -le 4 ] && is_err="true"
    echo '{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"t'"$i"'","name":"Bash","input":{"command":"x'"$i"'"}}]}}' >> "$WS/scout-stdout.log"
    echo '{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t'"$i"'","content":"r","is_error":'"$is_err"'}]}}' >> "$WS/scout-stdout.log"
done
echo '{"type":"result","subtype":"success","total_cost_usd":0.05,"num_turns":10,"usage":{}}' >> "$WS/scout-stdout.log"
run_observer "$WS" scout scout EVOLVE_OBSERVER_STALL_S=999 EVOLVE_OBSERVER_ERROR_RATE=0.3

REPORT="$WS/scout-observer-report.json"
EVENTS="$WS/scout-observer-events.ndjson"
spike_count=$(grep -c '"metric_type":"error_spike"' "$EVENTS" 2>/dev/null || echo 0)
[ "$spike_count" -ge "1" ] && pass "error_spike observation emitted (count=$spike_count)" || fail "no error_spike observation"
sev=$(grep '"metric_type":"error_spike"' "$EVENTS" | head -1 | jq -r '.severity')
[ "$sev" = "WARN" ] && pass "error_spike severity=WARN" || fail "spike severity expected WARN got $sev"
verdict=$(jq -r '.summary.verdict' "$REPORT" 2>/dev/null)
[ "$verdict" = "DEGRADED" ] && pass "verdict=DEGRADED (warns only)" || fail "verdict expected DEGRADED got $verdict"
rm -rf "$WS"

# ───────────────────────────────────────────────────────────────────────────
# TEST 5 — throttled: 3 rate_limit_event entries trigger WARN
header "TEST 5 — throttled: DEGRADED verdict, throttled metric"
WS=$(mktemp -d -t obs-test.XXXXXX)
: > "$WS/scout-stdout.log"
echo '{"type":"system","subtype":"init","session_id":"s-5","model":"sonnet"}' >> "$WS/scout-stdout.log"
echo '{"type":"rate_limit_event","message":"throttled #1"}' >> "$WS/scout-stdout.log"
echo '{"type":"rate_limit_event","message":"throttled #2"}' >> "$WS/scout-stdout.log"
echo '{"type":"rate_limit_event","message":"throttled #3"}' >> "$WS/scout-stdout.log"
echo '{"type":"result","subtype":"success","total_cost_usd":0.01,"num_turns":0,"usage":{}}' >> "$WS/scout-stdout.log"
run_observer "$WS" scout scout EVOLVE_OBSERVER_STALL_S=999 EVOLVE_OBSERVER_THROTTLE_N=3

REPORT="$WS/scout-observer-report.json"
EVENTS="$WS/scout-observer-events.ndjson"
thr_count=$(grep -c '"metric_type":"throttled"' "$EVENTS" 2>/dev/null || echo 0)
[ "$thr_count" -ge "1" ] && pass "throttled observation emitted (count=$thr_count)" || fail "no throttled observation"
sev=$(grep '"metric_type":"throttled"' "$EVENTS" | head -1 | jq -r '.severity')
[ "$sev" = "WARN" ] && pass "throttled severity=WARN" || fail "severity expected WARN got $sev"
rl=$(jq -r '.summary.rate_limit_events' "$REPORT" 2>/dev/null)
[ "$rl" = "3" ] && pass "rate_limit_events=3 in summary" || fail "rate_limit_events expected 3 got $rl"
rm -rf "$WS"

# ───────────────────────────────────────────────────────────────────────────
# TEST 6 — atomic write + envelope schema
header "TEST 6 — envelope schema: all events conform to unified envelope"
WS=$(mktemp -d -t obs-test.XXXXXX)
cat > "$WS/scout-stdout.log" <<'F'
{"type":"system","subtype":"init","session_id":"s-6","model":"sonnet"}
{"type":"result","subtype":"success","total_cost_usd":0.01,"num_turns":0,"usage":{}}
F
run_observer "$WS" scout scout EVOLVE_OBSERVER_STALL_S=999

EVENTS="$WS/scout-observer-events.ndjson"
# Every line must have: id, ts, trace_id, source, type, severity, data
fail_lines=0
while IFS= read -r line; do
    [ -z "$line" ] && continue
    if ! echo "$line" | jq -e '.id and .ts and .trace_id and .source and .type and .severity and .data' >/dev/null 2>&1; then
        fail_lines=$((fail_lines + 1))
    fi
done < "$EVENTS"
if [ "$fail_lines" = "0" ]; then
    pass "all envelopes have required fields (id, ts, trace_id, source, type, severity, data)"
else
    fail "$fail_lines events missing required envelope fields"
fi
# Source must include cycle, phase, agent, observer_pid
src_ok=$(head -1 "$EVENTS" | jq -e '.source | (.cycle and .phase and .agent and .observer_pid)' 2>/dev/null)
[ "$src_ok" = "true" ] && pass "source carries cycle/phase/agent/observer_pid" || fail "source schema incomplete"
rm -rf "$WS"

# ───────────────────────────────────────────────────────────────────────────
echo
echo "=== phase-observer-test.sh — $PASS/$TOTAL PASS ($FAIL fail) ==="
[ "$FAIL" = "0" ] && exit 0 || exit 1
