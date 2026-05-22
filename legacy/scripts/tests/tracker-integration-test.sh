#!/usr/bin/env bash
#
# tracker-integration-test.sh — End-to-end test for v10.5.0 Phase-B
# observability wiring. Exercises the FULL post-phase replay path:
#
#   stdout.log (NDJSON)
#     → tracker-writer.sh        (trackers/PHASE-ID.ndjson + trace.md + metrics/PHASE.json)
#     → append-phase-perf.sh     (Performance & Cost section appended to phase report)
#     → rollup-cycle-metrics.sh  (cycle-metrics.json)
#     → dashboard.sh --json      (machine-readable summary)
#
# Each test creates an isolated RUNS_DIR_OVERRIDE workspace so the test
# does not interact with real cycle data.
#
# Convention: "tracker-integration-test.sh — N/N PASS"

set -uo pipefail

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd -P)
PROJECT_ROOT=$(cd "$SCRIPT_DIR/.." && pwd -P)
OBSERVABILITY="$PROJECT_ROOT/observability"
TRACKER="$OBSERVABILITY/tracker-writer.sh"
APPEND_PERF="$OBSERVABILITY/append-phase-perf.sh"
ROLLUP="$OBSERVABILITY/rollup-cycle-metrics.sh"
DASHBOARD="$OBSERVABILITY/dashboard.sh"

for s in "$TRACKER" "$APPEND_PERF" "$ROLLUP" "$DASHBOARD"; do
    [ -f "$s" ] || { echo "FATAL: $s missing"; exit 1; }
done

TESTS_PASSED=0
TESTS_FAILED=0
TESTS_TOTAL=0

pass() { TESTS_PASSED=$((TESTS_PASSED + 1)); TESTS_TOTAL=$((TESTS_TOTAL + 1)); echo "  PASS: $1"; }
fail() { TESTS_FAILED=$((TESTS_FAILED + 1)); TESTS_TOTAL=$((TESTS_TOTAL + 1)); echo "  FAIL: $1"; }

# ---------- workspace helpers ----------
TMP_ROOT=""

setup_workspace() {
    TMP_ROOT=$(mktemp -d -t tracker-int-test.XXXXXX)
    mkdir -p "$TMP_ROOT/cycle-77"
    mkdir -p "$TMP_ROOT/cycle-76"  # baseline cycle for delta columns
    mkdir -p "$TMP_ROOT/cycle-75"
}

teardown_workspace() {
    [ -n "$TMP_ROOT" ] && rm -rf "$TMP_ROOT"
    TMP_ROOT=""
}

trap teardown_workspace EXIT INT TERM

# ---------- synthetic NDJSON fixture ----------
build_ndjson() {
    cat <<'NDJSON'
{"type":"system","subtype":"init","session_id":"sess-x1","model":"claude-sonnet-4-6"}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Reading state."}]}}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_1","name":"Read","input":{"file_path":".evolve/state.json"}}]}}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"{\"lastCycleNumber\":76}"}]}}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_2","name":"Bash","input":{"command":"ls .evolve/runs"}}]}}
{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"toolu_2","content":"cycle-75\ncycle-76\ncycle-77"}]}}
{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Done."}]}}
{"type":"result","subtype":"success","duration_ms":53400,"total_cost_usd":0.4256,"num_turns":6,"stop_reason":"end_turn","usage":{"input_tokens":2400,"output_tokens":850,"cache_read_input_tokens":18000,"cache_creation_input_tokens":4200}}
NDJSON
}

# Sidecar that append-phase-perf reads. Mimics what subagent-run.sh writes.
write_phase_sidecars() {
    local cycle="$1"
    local phase="$2"
    local cost_usd="$3"
    local duration_ms="$4"
    local turns="$5"

    local ws="$TMP_ROOT/cycle-$cycle"
    mkdir -p "$ws"

    # usage.json
    cat > "$ws/${phase}-usage.json" <<USAGE
{
  "duration_ms": $duration_ms,
  "num_turns": $turns,
  "total_cost_usd": $cost_usd,
  "usage": {
    "input_tokens": 2400,
    "output_tokens": 850,
    "cache_read_input_tokens": 18000,
    "cache_creation_input_tokens": 4200
  },
  "modelUsage": {
    "claude-sonnet-4-6": {"inputTokens": 2400, "outputTokens": 850}
  }
}
USAGE

    # timing.json
    cat > "$ws/${phase}-timing.json" <<TIMING
{
  "agent": "$phase",
  "cycle": $cycle,
  "profile_load_ms": 12,
  "prep_ms": 48,
  "adapter_invoke_ms": $duration_ms,
  "finalize_ms": 7,
  "total_ms": $((duration_ms + 67))
}
TIMING

    # report (stub)
    cat > "$ws/${phase}-report.md" <<REPORT
# ${phase} report — cycle $cycle

## Verdict
**PASS**

## Findings
(stub for integration test)
REPORT
}

# ---------- TEST 1: tracker-writer replay produces all 3 artifacts ----------
test_tracker_replay() {
    echo "→ Test 1: tracker-writer replay produces NDJSON + trace.md + metrics"
    setup_workspace
    write_phase_sidecars 77 scout 0.4256 53400 6

    local stdout_log="$TMP_ROOT/cycle-77/scout-stdout.log"
    build_ndjson > "$stdout_log"

    RUNS_DIR_OVERRIDE="$TMP_ROOT" cat "$stdout_log" | \
        RUNS_DIR_OVERRIDE="$TMP_ROOT" bash "$TRACKER" \
        --cycle=77 --phase=scout --invocation-id=scout-int-test \
        --runs-dir="$TMP_ROOT" \
        >/dev/null 2>/tmp/tracker-int.err
    rc=$?

    if [ "$rc" = "0" ]; then
        pass "tracker-writer exited 0"
    else
        fail "tracker-writer exited $rc (stderr below)"
        cat /tmp/tracker-int.err >&2
        teardown_workspace
        return
    fi

    local ndjson="$TMP_ROOT/cycle-77/.ephemeral/trackers/scout-scout-int-test.ndjson"
    local trace="$TMP_ROOT/cycle-77/.ephemeral/trace.md"
    local metrics="$TMP_ROOT/cycle-77/.ephemeral/metrics/scout.json"

    [ -s "$ndjson" ] && pass "NDJSON tracker written: $ndjson" || fail "NDJSON missing: $ndjson"
    [ -s "$trace" ] && pass "trace.md written" || fail "trace.md missing"
    [ -s "$metrics" ] && pass "metrics/scout.json written" || fail "metrics/scout.json missing"

    if [ -s "$metrics" ] && command -v jq >/dev/null 2>&1; then
        local cost
        cost=$(jq -r '.cost_usd // .total_cost_usd // 0' "$metrics" 2>/dev/null)
        if [ -n "$cost" ] && [ "$cost" != "0" ] && [ "$cost" != "null" ]; then
            pass "metrics include cost_usd ($cost)"
        else
            fail "metrics.cost_usd not populated (got: $cost)"
        fi
    fi

    teardown_workspace
}

# ---------- TEST 2: append-phase-perf appends section ----------
test_append_perf() {
    echo "→ Test 2: append-phase-perf appends '## Performance & Cost' section"
    setup_workspace

    # Write baseline sidecars for last 5 cycles so the delta columns populate.
    write_phase_sidecars 75 scout 0.50 60000 7
    write_phase_sidecars 76 scout 0.45 56000 6
    write_phase_sidecars 77 scout 0.4256 53400 6

    EVOLVE_PROJECT_ROOT="$TMP_ROOT/.." \
    RUNS_DIR_OVERRIDE="$TMP_ROOT" \
    bash "$APPEND_PERF" 77 scout >/tmp/perf-int.out 2>/tmp/perf-int.err
    rc=$?

    if [ "$rc" = "0" ]; then
        pass "append-phase-perf exited 0"
    else
        fail "append-phase-perf exited $rc"
        cat /tmp/perf-int.err >&2
        teardown_workspace
        return
    fi

    local report="$TMP_ROOT/cycle-77/scout-report.md"
    if grep -q "## Performance & Cost" "$report"; then
        pass "## Performance & Cost section present in report"
    else
        fail "## Performance & Cost section missing"
    fi

    # Idempotence: re-run should NOT duplicate the section.
    EVOLVE_PROJECT_ROOT="$TMP_ROOT/.." \
    RUNS_DIR_OVERRIDE="$TMP_ROOT" \
    bash "$APPEND_PERF" 77 scout >/dev/null 2>&1
    local n_sections
    n_sections=$(grep -c "^## Performance & Cost" "$report" || true)
    [ -z "$n_sections" ] && n_sections=0
    if [ "$n_sections" = "1" ]; then
        pass "append-phase-perf is idempotent (still 1 section after re-run)"
    else
        fail "append-phase-perf is not idempotent ($n_sections sections after re-run)"
    fi

    teardown_workspace
}

# ---------- TEST 3: rollup-cycle-metrics produces cycle-metrics.json ----------
test_rollup() {
    echo "→ Test 3: rollup-cycle-metrics produces cycle-metrics.json"
    setup_workspace
    write_phase_sidecars 77 scout 0.4256 53400 6
    write_phase_sidecars 77 auditor 0.1843 8800 3

    RUNS_DIR_OVERRIDE="$TMP_ROOT" bash "$ROLLUP" 77 \
        >/tmp/rollup-int.out 2>/tmp/rollup-int.err
    rc=$?

    if [ "$rc" = "0" ]; then
        pass "rollup-cycle-metrics exited 0"
    else
        fail "rollup-cycle-metrics exited $rc"
        cat /tmp/rollup-int.err >&2
        teardown_workspace
        return
    fi

    local metrics_file="$TMP_ROOT/cycle-77/.ephemeral/metrics/cycle-metrics.json"
    if [ -s "$metrics_file" ]; then
        pass "cycle-metrics.json written"
        if command -v jq >/dev/null 2>&1; then
            local total
            total=$(jq -r '.total_cost_usd // 0' "$metrics_file" 2>/dev/null)
            # 0.4256 + 0.1843 = 0.6099
            if [ -n "$total" ] && [ "$total" != "0" ] && [ "$total" != "null" ]; then
                pass "total_cost_usd populated ($total)"
            else
                fail "total_cost_usd not populated (got: $total)"
            fi

            local phases
            phases=$(jq -r '.phases | length' "$metrics_file" 2>/dev/null)
            if [ "$phases" = "2" ]; then
                pass "rollup has 2 phases (scout + auditor)"
            else
                fail "expected 2 phases, got: $phases"
            fi
        fi
    else
        fail "cycle-metrics.json missing"
    fi

    teardown_workspace
}

# ---------- TEST 4: dashboard.sh --json emits structured summary ----------
test_dashboard() {
    echo "→ Test 4: dashboard.sh --json emits structured summary"
    setup_workspace
    write_phase_sidecars 77 scout 0.4256 53400 6

    EVOLVE_PROJECT_ROOT="$TMP_ROOT/.." \
    RUNS_DIR_OVERRIDE="$TMP_ROOT" \
    bash "$DASHBOARD" --cycle=77 --json >/tmp/dash-int.out 2>/tmp/dash-int.err
    rc=$?

    if [ "$rc" = "0" ]; then
        pass "dashboard.sh exited 0"
    else
        fail "dashboard.sh exited $rc"
        cat /tmp/dash-int.err >&2
        teardown_workspace
        return
    fi

    if command -v jq >/dev/null 2>&1 && jq empty /tmp/dash-int.out 2>/dev/null; then
        pass "dashboard JSON is well-formed"
        local c
        c=$(jq -r '.cycle' /tmp/dash-int.out)
        if [ "$c" = "77" ]; then
            pass "dashboard reports cycle=77"
        else
            fail "dashboard reports cycle=$c (expected 77)"
        fi
        local a
        a=$(jq -r '.artifacts | length' /tmp/dash-int.out)
        if [ "$a" -gt 0 ] 2>/dev/null; then
            pass "dashboard discovered $a artifact(s)"
        else
            fail "dashboard reported 0 artifacts"
        fi
    else
        fail "dashboard JSON malformed (or jq missing)"
    fi

    teardown_workspace
}

# ---------- TEST 5: end-to-end pipeline (tracker → perf → rollup → dashboard) ----------
test_e2e_pipeline() {
    echo "→ Test 5: end-to-end pipeline composes correctly"
    setup_workspace
    write_phase_sidecars 75 scout 0.50 60000 7
    write_phase_sidecars 76 scout 0.45 56000 6
    write_phase_sidecars 77 scout 0.4256 53400 6
    build_ndjson > "$TMP_ROOT/cycle-77/scout-stdout.log"

    # Step 1: tracker-writer replay
    cat "$TMP_ROOT/cycle-77/scout-stdout.log" | bash "$TRACKER" \
        --cycle=77 --phase=scout --invocation-id=e2e --runs-dir="$TMP_ROOT" \
        >/dev/null 2>&1
    [ -s "$TMP_ROOT/cycle-77/.ephemeral/trackers/scout-e2e.ndjson" ] \
        && pass "step 1: tracker NDJSON produced" \
        || fail "step 1: tracker NDJSON missing"

    # Step 2: append-phase-perf
    EVOLVE_PROJECT_ROOT="$TMP_ROOT/.." RUNS_DIR_OVERRIDE="$TMP_ROOT" \
        bash "$APPEND_PERF" 77 scout >/dev/null 2>&1
    grep -q "## Performance & Cost" "$TMP_ROOT/cycle-77/scout-report.md" \
        && pass "step 2: perf section appended" \
        || fail "step 2: perf section missing"

    # Step 3: rollup
    RUNS_DIR_OVERRIDE="$TMP_ROOT" bash "$ROLLUP" 77 >/dev/null 2>&1
    [ -s "$TMP_ROOT/cycle-77/.ephemeral/metrics/cycle-metrics.json" ] \
        && pass "step 3: cycle-metrics.json written" \
        || fail "step 3: cycle-metrics.json missing"

    # Step 4: dashboard sees everything
    EVOLVE_PROJECT_ROOT="$TMP_ROOT/.." RUNS_DIR_OVERRIDE="$TMP_ROOT" \
        bash "$DASHBOARD" --cycle=77 --json > /tmp/e2e-dash.out 2>&1
    local has_metrics has_trace
    has_metrics=$(jq -r '.tracker.has_metrics' /tmp/e2e-dash.out 2>/dev/null)
    has_trace=$(jq -r '.tracker.has_trace' /tmp/e2e-dash.out 2>/dev/null)
    if [ "$has_metrics" = "yes" ] && [ "$has_trace" = "yes" ]; then
        pass "step 4: dashboard discovers metrics + trace"
    else
        fail "step 4: dashboard missing artifacts (metrics=$has_metrics trace=$has_trace)"
    fi

    teardown_workspace
}

# ---------- Run tests ----------
echo ""
echo "==========================================="
echo "tracker-integration-test.sh — v10.5.0"
echo "==========================================="

test_tracker_replay
test_append_perf
test_rollup
test_dashboard
test_e2e_pipeline

echo ""
echo "==========================================="
if [ "$TESTS_FAILED" = "0" ]; then
    echo "tracker-integration-test.sh — $TESTS_PASSED/$TESTS_TOTAL PASS (0 fail)"
    exit 0
else
    echo "tracker-integration-test.sh — $TESTS_PASSED/$TESTS_TOTAL PASS ($TESTS_FAILED fail)"
    exit 1
fi
