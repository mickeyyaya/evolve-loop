#!/usr/bin/env bash
#
# phase-watchdog-test.sh — Tests for scripts/dispatch/phase-watchdog.sh
#
# Tests:
#   1. Disable flag: EVOLVE_INACTIVITY_DISABLE=1 causes immediate rc=0 exit
#   2. Invalid args: no args causes rc=1
#   3. Stall detection: watchdog kills a sleeping stub process within timeout,
#      writes stall-progress.json, and checkpoints cycle-state.json
#
# Usage:
#   bash scripts/tests/phase-watchdog-test.sh
#
# Exit codes:
#   0 — all tests passed
#   1 — one or more tests failed

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WATCHDOG="${SCRIPT_DIR}/../dispatch/phase-watchdog.sh"
CYCLE_STATE_SH="${SCRIPT_DIR}/../lifecycle/cycle-state.sh"

PASS_COUNT=0
FAIL_COUNT=0

# ── Helpers ──────────────────────────────────────────────────────────────────

pass() {
    local name="$1"
    printf 'TEST: %s\nPASS\n\n' "$name"
    PASS_COUNT=$(( PASS_COUNT + 1 ))
}

fail() {
    local name="$1"
    local reason="$2"
    printf 'TEST: %s\nFAIL: %s\n\n' "$name" "$reason" >&2
    FAIL_COUNT=$(( FAIL_COUNT + 1 ))
}

# Return true if pgid still has any living processes.
pgid_has_procs() {
    local pgid="$1"
    ps -o pid,pgid 2>/dev/null | awk -v p="$pgid" '$2==p{found=1}END{exit !found}' 2>/dev/null
}

# ── Scratch directory setup ──────────────────────────────────────────────────

SCRATCH_BASE=""
cleanup() {
    [ -n "$SCRATCH_BASE" ] && rm -rf "$SCRATCH_BASE" 2>/dev/null || true
}
trap cleanup EXIT

SCRATCH_BASE="$(mktemp -d)"

# ── Sanity: watchdog script must exist ───────────────────────────────────────

if [ ! -f "$WATCHDOG" ]; then
    printf 'FATAL: watchdog script not found at %s\n' "$WATCHDOG" >&2
    exit 1
fi

# ── Minimal cycle-state JSON ─────────────────────────────────────────────────

CYCLE_STATE_JSON='{"cycle_id":99,"phase":"build","started_at":"2026-01-01T00:00:00Z","phase_started_at":"2026-01-01T00:00:00Z","active_agent":"builder","active_worktree":"/tmp/fake","completed_phases":["calibrate","research"],"workspace_path":".evolve/runs/cycle-99"}'

# ────────────────────────────────────────────────────────────────────────────
# Test 1: disable flag skips watchdog
# ────────────────────────────────────────────────────────────────────────────

t1_workspace="${SCRATCH_BASE}/t1-workspace"
mkdir -p "$t1_workspace"
t1_state="${SCRATCH_BASE}/t1-cycle-state.json"
printf '%s\n' "$CYCLE_STATE_JSON" > "$t1_state"

t1_start=$(date +%s)
EVOLVE_INACTIVITY_DISABLE=1 \
    EVOLVE_INACTIVITY_THRESHOLD_S=10 \
    EVOLVE_INACTIVITY_POLL_S=2 \
    EVOLVE_INACTIVITY_GRACE_S=3 \
    bash "$WATCHDOG" "$t1_workspace" "$$" 99 "$t1_state"
t1_rc=$?
t1_end=$(date +%s)
t1_elapsed=$(( t1_end - t1_start ))

if [ "$t1_rc" = "0" ] && [ "$t1_elapsed" -le 2 ]; then
    pass "disable flag skips watchdog"
else
    fail "disable flag skips watchdog" "expected rc=0 within 2s; got rc=${t1_rc} elapsed=${t1_elapsed}s"
fi

# ────────────────────────────────────────────────────────────────────────────
# Test 2: invalid args rejected
# ────────────────────────────────────────────────────────────────────────────

bash "$WATCHDOG" 2>/dev/null
t2_rc=$?

if [ "$t2_rc" = "1" ]; then
    pass "invalid args rejected"
else
    fail "invalid args rejected" "expected rc=1 for no-arg invocation; got rc=${t2_rc}"
fi

# ────────────────────────────────────────────────────────────────────────────
# Test 3: stall detection + PGID kill + checkpoint + artifacts
# ────────────────────────────────────────────────────────────────────────────

T3_NAME="stall detection + PGID kill + checkpoint + artifacts"

t3_workspace="${SCRATCH_BASE}/t3-workspace"
mkdir -p "$t3_workspace"

# Write cycle-state.json into a scratch .evolve dir so cycle-state.sh (if
# present and functional) writes into our scratch area, not the real project.
t3_project="${SCRATCH_BASE}/t3-project"
mkdir -p "${t3_project}/.evolve"
t3_state="${t3_project}/.evolve/cycle-state.json"
printf '%s\n' "$CYCLE_STATE_JSON" > "$t3_state"

# Write a log file into workspace so watchdog has a starting mtime reference.
printf 'stub started\n' > "${t3_workspace}/stub-stdout.log"

# Spawn stub in its own process group by enabling job control in this scope
# before forking. Without set -m, background subshells inherit the parent PGID,
# which would cause the watchdog to kill the test script itself when it fires.
set -m 2>/dev/null || true
(
    touch "${t3_workspace}/stub-stdout.log"
    sleep 60
) &
STUB_PID=$!
set +m 2>/dev/null || true

# Retrieve the PGID of the stub (may equal STUB_PID since it's a new group).
STUB_PGID=""
# Give the subshell a moment to settle.
sleep 0.3 2>/dev/null || sleep 1
if command -v ps >/dev/null 2>&1; then
    STUB_PGID=$(ps -o pgid= -p "$STUB_PID" 2>/dev/null | tr -d ' ') || STUB_PGID=""
fi
[ -z "$STUB_PGID" ] && STUB_PGID="$STUB_PID"

# Launch watchdog in background with short thresholds.
EVOLVE_INACTIVITY_THRESHOLD_S=10 \
    EVOLVE_INACTIVITY_POLL_S=2 \
    EVOLVE_INACTIVITY_GRACE_S=3 \
    EVOLVE_INACTIVITY_WARN_PCT=75 \
    EVOLVE_INACTIVITY_DISABLE=0 \
    EVOLVE_CYCLE_STATE_FILE="$t3_state" \
    EVOLVE_PROJECT_ROOT="$t3_project" \
    bash "$WATCHDOG" "$t3_workspace" "$STUB_PGID" 99 "$t3_state" &
WATCHDOG_PID=$!

# Poll for up to T+25s (threshold=10 + overhead) every 2s for all 4 conditions.
POLL_DEADLINE=$(( $(date +%s) + 35 ))
t3_checkpoint_ok=0
t3_stall_json_ok=0
t3_stub_dead=0
t3_watchdog_done=0

while [ "$(date +%s)" -lt "$POLL_DEADLINE" ]; do
    sleep 2

    # a) cycle-state.json contains "stall-inactivity"
    if [ "$t3_checkpoint_ok" = "0" ] && [ -f "$t3_state" ]; then
        state_contents="$(cat "$t3_state" 2>/dev/null || true)"
        if [[ "$state_contents" =~ stall-inactivity ]]; then
            t3_checkpoint_ok=1
        fi
    fi

    # b) stall-progress.json exists with valid JSON keys
    stall_progress="${t3_workspace}/stall-progress.json"
    if [ "$t3_stall_json_ok" = "0" ] && [ -f "$stall_progress" ]; then
        sp_contents="$(cat "$stall_progress" 2>/dev/null || true)"
        if [[ "$sp_contents" =~ idle_s ]] && [[ "$sp_contents" =~ threshold_s ]] && [[ "$sp_contents" =~ checkpoint_ts ]]; then
            t3_stall_json_ok=1
        fi
    fi

    # c) stub process is dead
    if [ "$t3_stub_dead" = "0" ]; then
        if ! kill -0 "$STUB_PID" 2>/dev/null; then
            t3_stub_dead=1
        fi
    fi

    # d) watchdog itself has exited
    if [ "$t3_watchdog_done" = "0" ]; then
        if ! kill -0 "$WATCHDOG_PID" 2>/dev/null; then
            wait "$WATCHDOG_PID" 2>/dev/null || true
            t3_watchdog_done=1
        fi
    fi

    # All 4 conditions met?
    if [ "$t3_checkpoint_ok" = "1" ] && \
       [ "$t3_stall_json_ok" = "1" ] && \
       [ "$t3_stub_dead" = "1" ] && \
       [ "$t3_watchdog_done" = "1" ]; then
        break
    fi
done

# Ensure watchdog is gone before we evaluate (collect exit code).
if kill -0 "$WATCHDOG_PID" 2>/dev/null; then
    kill "$WATCHDOG_PID" 2>/dev/null || true
    wait "$WATCHDOG_PID" 2>/dev/null || true
fi
# Ensure stub is gone.
if kill -0 "$STUB_PID" 2>/dev/null; then
    kill "$STUB_PID" 2>/dev/null || true
    wait "$STUB_PID" 2>/dev/null || true
fi

# Evaluate results.
t3_fail_reasons=""

if [ "$t3_checkpoint_ok" = "0" ]; then
    t3_fail_reasons="${t3_fail_reasons}; cycle-state.json did not contain 'stall-inactivity'"
fi
if [ "$t3_stall_json_ok" = "0" ]; then
    t3_fail_reasons="${t3_fail_reasons}; stall-progress.json missing or invalid"
fi
if [ "$t3_stub_dead" = "0" ]; then
    t3_fail_reasons="${t3_fail_reasons}; stub process still alive (pid=$STUB_PID)"
fi
if [ "$t3_watchdog_done" = "0" ]; then
    t3_fail_reasons="${t3_fail_reasons}; watchdog did not exit within deadline"
fi

if [ -z "$t3_fail_reasons" ]; then
    pass "$T3_NAME"
else
    fail "$T3_NAME" "${t3_fail_reasons## ; }"
fi


# ── Test 4 — v9.2.0: incremental stdout.log writes prevent FIRE ──────────────
# This is the inverse of Test 3. When the subagent is genuinely working and
# producing stream-json events (each event = one mtime touch on stdout.log),
# the watchdog must NOT fire — even when the threshold has long elapsed since
# the stub PROCESS started. Validates the architectural fix from v9.2.0 where
# claude.sh emits --output-format=stream-json so stdout.log gets continuous
# mtime updates throughout a phase.

T4_NAME="incremental log writes prevent FIRE (stream-json liveness)"

t4_workspace="${SCRATCH_BASE}/t4-workspace"
mkdir -p "$t4_workspace"

t4_project="${SCRATCH_BASE}/t4-project"
mkdir -p "${t4_project}/.evolve"
t4_state="${t4_project}/.evolve/cycle-state.json"
printf '%s\n' "$CYCLE_STATE_JSON" > "$t4_state"

# Start an "active subagent" simulator: touches stdout.log every 3s for 25s.
# Threshold is 10s. If watchdog watched only end-of-run output, it would
# fire at 10s. With incremental writes, the most-recent mtime stays within
# 3s of `now`, so the FIRE condition never triggers.
printf 'active subagent simulator: start\n' > "${t4_workspace}/active-stdout.log"

set -m 2>/dev/null || true
(
    # Touch the log every 3 seconds for 25 seconds. Each touch updates mtime.
    end_time=$(( $(date +%s) + 25 ))
    while [ "$(date +%s)" -lt "$end_time" ]; do
        printf '{"type":"event","seq":%d}\n' "$(date +%s)" >> "${t4_workspace}/active-stdout.log"
        sleep 3
    done
) &
ACTIVE_PID=$!
set +m 2>/dev/null || true

sleep 0.3
ACTIVE_PGID=""
if command -v ps >/dev/null 2>&1; then
    ACTIVE_PGID=$(ps -o pgid= -p "$ACTIVE_PID" 2>/dev/null | tr -d ' ') || ACTIVE_PGID=""
fi
[ -z "$ACTIVE_PGID" ] && ACTIVE_PGID="$ACTIVE_PID"

# Launch watchdog with 10s threshold. The watchdog should run the full 25s
# without firing, because the simulator keeps stdout.log mtime fresh.
EVOLVE_INACTIVITY_THRESHOLD_S=10 \
    EVOLVE_INACTIVITY_POLL_S=2 \
    EVOLVE_INACTIVITY_GRACE_S=3 \
    EVOLVE_INACTIVITY_WARN_PCT=75 \
    EVOLVE_INACTIVITY_DISABLE=0 \
    EVOLVE_CYCLE_STATE_FILE="$t4_state" \
    EVOLVE_PROJECT_ROOT="$t4_project" \
    bash "$WATCHDOG" "$t4_workspace" "$ACTIVE_PGID" 99 "$t4_state" &
WATCHDOG_PID=$!

# Wait long enough that the watchdog would have fired (3x threshold) if it
# were going to. The simulator runs for 25s; sleep 22s and then check.
sleep 22

# Did the watchdog fire? (Indicators: stall-progress.json + checkpoint).
t4_no_stall_json=1
t4_stub_alive=1
t4_no_checkpoint=1

if [ -f "${t4_workspace}/stall-progress.json" ]; then
    t4_no_stall_json=0
fi

if ! kill -0 "$ACTIVE_PID" 2>/dev/null; then
    # Note: the simulator naturally exits after 25s. If we're past that point,
    # this isn't a failure. Allow up to 27s total elapsed.
    elapsed=$(( $(date +%s) - $(stat -f %B "$t4_workspace" 2>/dev/null || echo 0) ))
    if [ "$elapsed" -lt 25 ]; then
        t4_stub_alive=0
    fi
fi

if [ -f "$t4_state" ] && grep -q "stall-inactivity" "$t4_state" 2>/dev/null; then
    t4_no_checkpoint=0
fi

# Cleanup.
if kill -0 "$ACTIVE_PID" 2>/dev/null; then
    kill -TERM "$ACTIVE_PID" 2>/dev/null || true
    wait "$ACTIVE_PID" 2>/dev/null || true
fi
if kill -0 "$WATCHDOG_PID" 2>/dev/null; then
    kill -TERM "$WATCHDOG_PID" 2>/dev/null || true
    wait "$WATCHDOG_PID" 2>/dev/null || true
fi

t4_fail_reasons=""
if [ "$t4_no_stall_json" = "0" ]; then
    t4_fail_reasons="${t4_fail_reasons}; stall-progress.json was created (watchdog fired despite active writes)"
fi
if [ "$t4_stub_alive" = "0" ]; then
    t4_fail_reasons="${t4_fail_reasons}; active simulator was killed by watchdog (false-positive SIGTERM)"
fi
if [ "$t4_no_checkpoint" = "0" ]; then
    t4_fail_reasons="${t4_fail_reasons}; cycle-state.json shows stall-inactivity checkpoint (false fire)"
fi

if [ -z "$t4_fail_reasons" ]; then
    pass "$T4_NAME"
else
    fail "$T4_NAME" "${t4_fail_reasons## ; }"
fi

# ── Summary ──────────────────────────────────────────────────────────────────

echo "---"
printf 'Results: %d passed, %d failed\n' "$PASS_COUNT" "$FAIL_COUNT"

if [ "$FAIL_COUNT" -eq 0 ]; then
    exit 0
else
    exit 1
fi
