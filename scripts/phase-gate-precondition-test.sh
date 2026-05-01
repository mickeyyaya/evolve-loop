#!/usr/bin/env bash
#
# phase-gate-precondition-test.sh — Unit tests for
# scripts/guards/phase-gate-precondition.sh (v8.13.1).
#
# Tests: trigger detection (only `bash scripts/subagent-run.sh`), per-phase
# expected-agent allowlist, re-spawn handling, no-cycle passthrough, bypass.
#
# Usage: bash scripts/phase-gate-precondition-test.sh
# Exit 0 = all pass; non-zero = failures.

set -uo pipefail

unset EVOLVE_BYPASS_PHASE_GATE

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GATE="$REPO_ROOT/scripts/guards/phase-gate-precondition.sh"
HELPER="$REPO_ROOT/scripts/cycle-state.sh"

TEST_STATE_DIR=$(mktemp -d -t pgpre-test.XXXXXX)
TEST_STATE="$TEST_STATE_DIR/cycle-state.json"
trap 'rm -rf "$TEST_STATE_DIR"' EXIT
export EVOLVE_CYCLE_STATE_FILE="$TEST_STATE"

PASS=0; FAIL=0; TESTS_TOTAL=0
pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail()   { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; TESTS_TOTAL=$((TESTS_TOTAL + 1)); }

run_gate() {
    local payload="$1"
    local extra_env="${2:-}"
    if [ -n "$extra_env" ]; then
        env $extra_env EVOLVE_CYCLE_STATE_FILE="$TEST_STATE" bash "$GATE" <<< "$payload" >/dev/null 2>&1
    else
        bash "$GATE" <<< "$payload" >/dev/null 2>&1
    fi
}
expect_allow() {
    local label="$1" payload="$2" extra="${3:-}"
    set +e; run_gate "$payload" "$extra"; local rc=$?; set -e
    if [ "$rc" = "0" ]; then pass "$label (rc=0)"
    else fail "$label — expected rc=0, got rc=$rc"; fi
}
expect_deny() {
    local label="$1" payload="$2" extra="${3:-}"
    set +e; run_gate "$payload" "$extra"; local rc=$?; set -e
    if [ "$rc" = "2" ]; then pass "$label (rc=2)"
    else fail "$label — expected rc=2, got rc=$rc"; fi
}

set_state() {
    local phase="$1" agent="${2:-}"
    rm -f "$TEST_STATE"
    bash "$HELPER" init 99000 .evolve/runs/cycle-99000 >/dev/null
    if [ "$phase" != "calibrate" ] || [ -n "$agent" ]; then
        local agent_arg='null'; [ -n "$agent" ] && agent_arg="\"$agent\""
        jq -c \
            --arg phase "$phase" \
            --argjson agent "$agent_arg" \
            '.phase = $phase | .active_agent = $agent' \
            "$TEST_STATE" > "$TEST_STATE.tmp" && mv "$TEST_STATE.tmp" "$TEST_STATE"
    fi
}

# === Test 1: non-subagent-run command → ALLOW (passthrough) ===================
header "Test 1: non-subagent-run command → ALLOW"
set_state build builder
expect_allow "ls -la" '{"tool_input":{"command":"ls -la"}}'

# === Test 2: bash scripts/ship.sh → ALLOW (not our trigger) ===================
header "Test 2: bash scripts/ship.sh → ALLOW (passthrough)"
set_state build builder
expect_allow "ship.sh invocation" '{"tool_input":{"command":"bash scripts/ship.sh \"feat: x\""}}'

# === Test 3: no cycle-state → ALLOW (ad-hoc invocation) =======================
header "Test 3: no cycle-state → ALLOW"
rm -f "$TEST_STATE"
expect_allow "ad-hoc subagent-run" \
    '{"tool_input":{"command":"bash scripts/subagent-run.sh scout 99 .evolve/runs/cycle-99"}}'

# === Test 4: phase=calibrate, agent=scout → ALLOW =============================
header "Test 4: phase=calibrate + scout → ALLOW"
set_state calibrate ""
expect_allow "calibrate→scout" \
    '{"tool_input":{"command":"bash scripts/subagent-run.sh scout 99000 .evolve/runs/cycle-99000"}}'

# === Test 5: phase=calibrate, agent=builder → DENY ============================
header "Test 5: phase=calibrate + builder → DENY (out of order)"
set_state calibrate ""
expect_deny "calibrate→builder" \
    '{"tool_input":{"command":"bash scripts/subagent-run.sh builder 99000 .evolve/runs/cycle-99000"}}'

# === Test 6: phase=build, agent=auditor → ALLOW ===============================
header "Test 6: phase=build + auditor → ALLOW (next phase)"
set_state build builder
expect_allow "build→auditor" \
    '{"tool_input":{"command":"bash scripts/subagent-run.sh auditor 99000 .evolve/runs/cycle-99000"}}'

# === Test 7: phase=build, agent=builder (re-spawn) → ALLOW ====================
header "Test 7: phase=build + builder re-spawn → ALLOW"
set_state build builder
expect_allow "build→builder re-spawn" \
    '{"tool_input":{"command":"bash scripts/subagent-run.sh builder 99000 .evolve/runs/cycle-99000"}}'

# === Test 8: phase=build, agent=scout → DENY (going backwards) ================
header "Test 8: phase=build + scout → DENY (going backwards)"
set_state build builder
expect_deny "build→scout (backwards)" \
    '{"tool_input":{"command":"bash scripts/subagent-run.sh scout 99000 .evolve/runs/cycle-99000"}}'

# === Test 9: phase=audit, agent=retrospective → ALLOW =========================
header "Test 9: phase=audit + retrospective → ALLOW"
set_state audit auditor
expect_allow "audit→retrospective" \
    '{"tool_input":{"command":"bash scripts/subagent-run.sh retrospective 99000 .evolve/runs/cycle-99000"}}'

# === Test 10: phase=audit, agent=builder → DENY ===============================
header "Test 10: phase=audit + builder → DENY (cannot revert)"
set_state audit auditor
expect_deny "audit→builder" \
    '{"tool_input":{"command":"bash scripts/subagent-run.sh builder 99000 .evolve/runs/cycle-99000"}}'

# === Test 11: bypass env → ALLOW even when would deny =========================
header "Test 11: EVOLVE_BYPASS_PHASE_GATE=1 → ALLOW"
set_state calibrate ""
expect_allow "bypass + builder during calibrate" \
    '{"tool_input":{"command":"bash scripts/subagent-run.sh builder 99000 .evolve/runs/cycle-99000"}}' \
    "EVOLVE_BYPASS_PHASE_GATE=1"

# === Test 12: unrecognized agent name → ALLOW (delegate) ======================
header "Test 12: unrecognized agent name → ALLOW (delegate to subagent-run.sh)"
set_state build builder
expect_allow "unknown agent 'foobar'" \
    '{"tool_input":{"command":"bash scripts/subagent-run.sh foobar 99000 .evolve/runs/cycle-99000"}}'

# === Test 13: empty payload → ALLOW (manual invocation) =======================
header "Test 13: empty payload → ALLOW"
expect_allow "empty payload" ""

# === Test 14: phase=discover, agent=builder → ALLOW (forward to next) =========
header "Test 14: phase=discover + builder → ALLOW (next phase)"
set_state discover scout
expect_allow "discover→builder" \
    '{"tool_input":{"command":"bash scripts/subagent-run.sh builder 99000 .evolve/runs/cycle-99000"}}'

# === Test 15: leading whitespace + tabs in command → still parsed correctly ===
header "Test 15: leading whitespace handled"
set_state build builder
expect_allow "leading-ws + auditor" \
    '{"tool_input":{"command":"   bash scripts/subagent-run.sh auditor 99000 .evolve/runs/cycle-99000"}}'

# === Test 16: scout-worker-* allowed in research phase (parent role check) ===
# Workers (Sprint 1 fan-out) should be sequence-checked against their parent
# role's expected-agent set.
header "Test 16: research + scout-worker-codebase → ALLOW (worker for valid parent)"
set_state research scout
expect_allow "research→scout-worker-codebase" \
    '{"tool_input":{"command":"bash scripts/subagent-run.sh scout-worker-codebase 99000 .evolve/runs/cycle-99000"}}'

# === Test 17: auditor-worker-* allowed in audit phase ========================
header "Test 17: audit + auditor-worker-eval → ALLOW"
set_state audit auditor
expect_allow "audit→auditor-worker-eval" \
    '{"tool_input":{"command":"bash scripts/subagent-run.sh auditor-worker-eval 99000 .evolve/runs/cycle-99000"}}'

# === Test 18: scout-worker-* denied in build phase (out of order) ============
header "Test 18: build + scout-worker-codebase → DENY (worker for wrong-phase parent)"
set_state build builder
expect_deny "build→scout-worker-codebase" \
    '{"tool_input":{"command":"bash scripts/subagent-run.sh scout-worker-codebase 99000 .evolve/runs/cycle-99000"}}'

# === Test 19: worker re-spawn match against active_agent =====================
# When active_agent=scout and a worker scout-worker-codebase is requested,
# the prefix should re-spawn-match (active_agent is the parent role).
header "Test 19: active=scout + scout-worker-research → ALLOW (re-spawn prefix)"
set_state research scout
expect_allow "research→scout-worker-research" \
    '{"tool_input":{"command":"bash scripts/subagent-run.sh scout-worker-research 99000 .evolve/runs/cycle-99000"}}'

# === Summary =================================================================
echo
echo "=========================================="
echo "  Total tests: $TESTS_TOTAL"
echo "  Passed:      $PASS"
echo "  Failed:      $FAIL"
echo "=========================================="

[ "$FAIL" = "0" ] && exit 0 || exit 1
