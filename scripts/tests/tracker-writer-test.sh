#!/usr/bin/env bash
#
# tracker-writer-test.sh — Unit + integration tests for tracker-writer.sh.
#
# Synthetic stream-json fixtures fed via stdin; no live claude -p calls.
# Each test exits non-zero on assertion failure; the runner accumulates a
# pass/fail count and exits accordingly.
#
# Convention: "bash scripts/tests/tracker-writer-test.sh — N/N PASS"

set -uo pipefail

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd -P)
PROJECT_ROOT=$(cd "$SCRIPT_DIR/.." && pwd -P)
TRACKER="$PROJECT_ROOT/observability/tracker-writer.sh"
[ -f "$TRACKER" ] || { echo "FATAL: $TRACKER not found"; exit 1; }

TESTS_PASSED=0
TESTS_FAILED=0
TESTS_TOTAL=0

pass() { TESTS_PASSED=$((TESTS_PASSED + 1)); TESTS_TOTAL=$((TESTS_TOTAL + 1)); echo "  PASS: $1"; }
fail() { TESTS_FAILED=$((TESTS_FAILED + 1)); TESTS_TOTAL=$((TESTS_TOTAL + 1)); echo "  FAIL: $1"; }

# Each test creates an isolated temp workspace.
new_workspace() {
    local d
    d=$(mktemp -d -t tracker-test.XXXXXX)
    mkdir -p "$d/cycle-99"
    echo "$d"
}

cleanup_workspace() { rm -rf "$1"; }

# Synthetic stream-json fixture builder.
build_fixture_basic() {
    cat << 'F1'
{"type":"system","subtype":"init","session_id":"sess-001","model":"sonnet"}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Starting analysis."}]}}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_A1","name":"Read","input":{"file_path":".evolve/state.json"}}]}}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_A1","content":"{\"lastCycleNumber\":35}"}]}}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_A2","name":"Bash","input":{"command":"ls .evolve/inbox"}}]}}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_A2","content":"file1.json\nfile2.json"}]}}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Done."}]}}
{"type":"result","subtype":"success","duration_ms":12345,"total_cost_usd":0.0123,"num_turns":4,"stop_reason":"end_turn","usage":{"input_tokens":100,"output_tokens":50}}
F1
}

build_fixture_malformed() {
    # Line 2 is not valid JSON; tracker must continue.
    cat << 'F2'
{"type":"system","subtype":"init","session_id":"sess-002","model":"sonnet"}
this is not valid json at all
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Ignoring bad line."}]}}
{"type":"result","subtype":"success","duration_ms":1000,"total_cost_usd":0.0001,"num_turns":1,"stop_reason":"end_turn"}
F2
}

build_fixture_empty() {
    # No events at all.
    echo -n ""
}

build_fixture_error() {
    cat << 'F3'
{"type":"system","subtype":"init","session_id":"sess-err","model":"sonnet"}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_E1","name":"WebSearch","input":{"query":"x"}}]}}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_E1","content":"network error","is_error":true}]}}
{"type":"error","message":"connection refused","details":{"code":"ECONNREFUSED"}}
{"type":"result","subtype":"error","duration_ms":5000,"total_cost_usd":0,"num_turns":1,"stop_reason":"error"}
F3
}

echo "=== tracker-writer-test.sh ==="
echo ""

# ─────────────────────────────────────────────────────────────────────────────
echo "TEST 1: basic happy path"
WS=$(new_workspace)
build_fixture_basic | bash "$TRACKER" --cycle=99 --phase=scout --invocation-id=test001 --runs-dir="$WS" >/dev/null 2>&1
RC=$?
TRACKER_FILE="$WS/cycle-99/.ephemeral/trackers/scout-test001.ndjson"
TRACE_FILE="$WS/cycle-99/.ephemeral/trace.md"
METRICS_FILE="$WS/cycle-99/.ephemeral/metrics/scout.json"

if [ "$RC" = "0" ]; then pass "exit code 0"; else fail "exit code expected 0, got $RC"; fi
if [ -f "$TRACKER_FILE" ]; then pass "tracker NDJSON created"; else fail "tracker NDJSON missing"; fi
if [ -f "$TRACE_FILE" ]; then pass "trace.md created"; else fail "trace.md missing"; fi
if [ -f "$METRICS_FILE" ]; then pass "metrics.json created"; else fail "metrics.json missing"; fi

# NDJSON must have 8 lines (matching input events).
NDJSON_LINES=$(wc -l < "$TRACKER_FILE" | tr -d ' ')
if [ "$NDJSON_LINES" = "8" ]; then pass "NDJSON has 8 events"; else fail "NDJSON expected 8 lines, got $NDJSON_LINES"; fi

# Each NDJSON line must have ts, cycle, phase, invocation_id.
SAMPLE=$(head -1 "$TRACKER_FILE")
if echo "$SAMPLE" | jq -e '.ts and .cycle and .phase and .invocation_id' >/dev/null 2>&1; then
    pass "NDJSON entries carry envelope fields"
else
    fail "NDJSON envelope fields missing"
fi

# Metrics.json structure.
if jq -e '.schema_version == "1.0" and .cycle == 99 and .phase == "scout"' "$METRICS_FILE" >/dev/null 2>&1; then
    pass "metrics.json has correct cycle+phase"
else
    fail "metrics.json structure wrong"
fi

# Cost from the result event should propagate.
M_COST=$(jq -r '.cost_usd' "$METRICS_FILE")
if [ "$M_COST" = "0.0123" ]; then pass "cost_usd = 0.0123 from result event"; else fail "cost_usd expected 0.0123, got $M_COST"; fi

# Tool histogram should contain Read and Bash.
TOOL_COUNT=$(jq '.tool_calls | length' "$METRICS_FILE")
if [ "$TOOL_COUNT" = "2" ]; then pass "tool_calls has 2 distinct tools"; else fail "tool_calls expected 2, got $TOOL_COUNT"; fi

# trace.md has START + END markers.
if grep -q "START" "$TRACE_FILE" && grep -q "END" "$TRACE_FILE"; then
    pass "trace.md has START + END markers"
else
    fail "trace.md missing START/END markers"
fi

cleanup_workspace "$WS"
echo ""

# ─────────────────────────────────────────────────────────────────────────────
echo "TEST 2: malformed JSON line is skipped, not fatal"
WS=$(new_workspace)
ERR_OUT=$(build_fixture_malformed | bash "$TRACKER" --cycle=99 --phase=scout --invocation-id=test002 --runs-dir="$WS" 2>&1 >/dev/null)
RC=$?

if [ "$RC" = "0" ]; then pass "tracker exits 0 despite bad line"; else fail "tracker should not fatal on bad line; got rc=$RC"; fi

if echo "$ERR_OUT" | grep -q "skipping non-JSON"; then
    pass "bad line logged to stderr"
else
    fail "expected 'skipping non-JSON' warning"
fi

# Only 3 valid events should land in NDJSON.
TRACKER_FILE="$WS/cycle-99/.ephemeral/trackers/scout-test002.ndjson"
NDJSON_LINES=$(wc -l < "$TRACKER_FILE" | tr -d ' ')
if [ "$NDJSON_LINES" = "3" ]; then pass "only 3 valid events in NDJSON"; else fail "expected 3 events, got $NDJSON_LINES"; fi

cleanup_workspace "$WS"
echo ""

# ─────────────────────────────────────────────────────────────────────────────
echo "TEST 3: empty input produces empty NDJSON + valid metrics"
WS=$(new_workspace)
build_fixture_empty | bash "$TRACKER" --cycle=99 --phase=scout --invocation-id=test003 --runs-dir="$WS" >/dev/null 2>&1
RC=$?

if [ "$RC" = "0" ]; then pass "empty input exits 0"; else fail "empty input rc=$RC"; fi

METRICS_FILE="$WS/cycle-99/.ephemeral/metrics/scout.json"
if [ -f "$METRICS_FILE" ] && jq -e '.event_count == 0' "$METRICS_FILE" >/dev/null 2>&1; then
    pass "metrics.json shows event_count=0"
else
    fail "metrics.json missing or wrong event_count"
fi

cleanup_workspace "$WS"
echo ""

# ─────────────────────────────────────────────────────────────────────────────
echo "TEST 4: error event surfaces in trace.md"
WS=$(new_workspace)
build_fixture_error | bash "$TRACKER" --cycle=99 --phase=scout --invocation-id=test004 --runs-dir="$WS" >/dev/null 2>&1
RC=$?

if [ "$RC" = "0" ]; then pass "error fixture exits 0"; else fail "error fixture rc=$RC"; fi

TRACE_FILE="$WS/cycle-99/.ephemeral/trace.md"
if grep -q "ERROR" "$TRACE_FILE" 2>/dev/null; then
    pass "ERROR marker in trace.md"
else
    fail "ERROR not surfaced in trace.md"
fi

METRICS_FILE="$WS/cycle-99/.ephemeral/metrics/scout.json"
STOP=$(jq -r '.stop_reason' "$METRICS_FILE")
if [ "$STOP" = "error" ]; then pass "stop_reason=error captured"; else fail "stop_reason expected 'error', got '$STOP'"; fi

cleanup_workspace "$WS"
echo ""

# ─────────────────────────────────────────────────────────────────────────────
echo "TEST 5: required flag enforcement"
WS=$(new_workspace)

# Missing --cycle
echo "{}" | bash "$TRACKER" --phase=scout --invocation-id=test005 --runs-dir="$WS" >/dev/null 2>&1
RC=$?
if [ "$RC" = "10" ]; then pass "missing --cycle exits 10"; else fail "expected rc=10 for missing --cycle, got $RC"; fi

# Missing --phase
echo "{}" | bash "$TRACKER" --cycle=99 --invocation-id=test005 --runs-dir="$WS" >/dev/null 2>&1
RC=$?
if [ "$RC" = "10" ]; then pass "missing --phase exits 10"; else fail "expected rc=10 for missing --phase, got $RC"; fi

# Missing --invocation-id
echo "{}" | bash "$TRACKER" --cycle=99 --phase=scout --runs-dir="$WS" >/dev/null 2>&1
RC=$?
if [ "$RC" = "10" ]; then pass "missing --invocation-id exits 10"; else fail "expected rc=10 for missing --invocation-id, got $RC"; fi

# Bad cycle value
echo "{}" | bash "$TRACKER" --cycle=foo --phase=scout --invocation-id=test005 --runs-dir="$WS" >/dev/null 2>&1
RC=$?
if [ "$RC" = "10" ]; then pass "non-numeric --cycle exits 10"; else fail "expected rc=10 for bad --cycle, got $RC"; fi

cleanup_workspace "$WS"
echo ""

# ─────────────────────────────────────────────────────────────────────────────
echo "=== tracker-writer-test.sh — $TESTS_PASSED/$TESTS_TOTAL PASS ==="

if [ "$TESTS_FAILED" -gt 0 ]; then
    exit 1
fi
