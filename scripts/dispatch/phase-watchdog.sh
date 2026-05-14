#!/usr/bin/env bash
#
# phase-watchdog.sh — Activity-based phase watchdog.
#
# Background script that polls file mtimes within a workspace and kills a
# stalled process group when no file activity has been detected for longer
# than EVOLVE_INACTIVITY_THRESHOLD_S seconds.
#
# Usage:
#   phase-watchdog.sh <workspace> <target_pgid> <cycle> <cycle_state_path>
#
# Arguments:
#   <workspace>         — absolute path to the cycle's workspace directory
#   <target_pgid>       — process group ID to kill on stall detection
#   <cycle>             — current cycle number (integer)
#   <cycle_state_path>  — absolute path to cycle-state.json
#
# Env vars (with defaults):
#   EVOLVE_INACTIVITY_THRESHOLD_S   — stall threshold in seconds (default: 240)
#   EVOLVE_INACTIVITY_POLL_S        — poll interval in seconds (default: 15)
#   EVOLVE_INACTIVITY_WARN_PCT      — warn threshold as % of threshold (default: 75)
#   EVOLVE_INACTIVITY_GRACE_S       — grace period between TERM and KILL (default: 10)
#   EVOLVE_INACTIVITY_DISABLE       — set to 1 to disable watchdog entirely (default: 0)
#   EVOLVE_PROJECT_ROOT             — project root for locating ledger.jsonl
#
# Exit codes:
#   0 — watchdog fired and completed kill sequence, OR disabled via env var
#   1 — invalid arguments or workspace missing

set -uo pipefail

THRESHOLD_S="${EVOLVE_INACTIVITY_THRESHOLD_S:-240}"
POLL_S="${EVOLVE_INACTIVITY_POLL_S:-15}"
WARN_PCT="${EVOLVE_INACTIVITY_WARN_PCT:-75}"
GRACE_S="${EVOLVE_INACTIVITY_GRACE_S:-10}"
DISABLE="${EVOLVE_INACTIVITY_DISABLE:-0}"

_log() {
    printf '[phase-watchdog] %s\n' "$*" >&2
}
# abnormal-events.jsonl: append stall-detected event (best-effort).
_append_abnormal_event() {
    local _ws="$1" _det="$2"
    [ -d "$_ws" ] || return 0
    local _ts; _ts=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    local _det_esc; _det_esc=$(printf '%s' "$_det" | sed 's/"/\\"/g')
    printf '{"event_type":"stall-detected","timestamp":"%s","source_phase":"phase-watchdog","severity":"HIGH","details":"%s","remediation_hint":"Check agent turn count; reduce scope or increase EVOLVE_INACTIVITY_THRESHOLD_S"}\n' \
        "$_ts" "$_det_esc" >> "$_ws/abnormal-events.jsonl" 2>/dev/null || true
}

# Portable mtime: try BSD stat first (macOS), then GNU stat, fallback to 0.
get_mtime() {
    local f="$1"
    [ -e "$f" ] || { printf '0'; return 0; }
    local m
    m=$(stat -f %m "$f" 2>/dev/null) && printf '%s' "$m" && return 0
    m=$(stat -c %Y "$f" 2>/dev/null) && printf '%s' "$m" && return 0
    printf '0'
}

# Scan a glob pattern, return the newest mtime found and the corresponding path.
# Outputs two lines: <mtime> <path>
_scan_glob() {
    local pattern="$1"
    local best_mtime=0
    local best_path=""
    local f
    for f in $pattern; do
        [ -e "$f" ] || continue
        local m
        m=$(get_mtime "$f")
        if [ "$m" -gt "$best_mtime" ] 2>/dev/null; then
            best_mtime="$m"
            best_path="$f"
        fi
    done
    printf '%s %s\n' "$best_mtime" "$best_path"
}

# ── Argument validation ──────────────────────────────────────────────────────

if [ $# -lt 4 ]; then
    _log "ERROR: requires 4 arguments: <workspace> <target_pgid> <cycle> <cycle_state_path>"
    exit 1
fi

WORKSPACE="$1"
TARGET_PGID="$2"
CYCLE="$3"
CYCLE_STATE_PATH="$4"

if [ ! -d "$WORKSPACE" ]; then
    _log "ERROR: workspace directory does not exist: $WORKSPACE"
    exit 1
fi

if ! [[ "$TARGET_PGID" =~ ^[0-9]+$ ]]; then
    _log "ERROR: target_pgid must be a positive integer, got: $TARGET_PGID"
    exit 1
fi

if ! [[ "$CYCLE" =~ ^[0-9]+$ ]]; then
    _log "ERROR: cycle must be a positive integer, got: $CYCLE"
    exit 1
fi

# ── Disable check ────────────────────────────────────────────────────────────

if [ "$DISABLE" = "1" ]; then
    _log "EVOLVE_INACTIVITY_DISABLE=1 — watchdog disabled, exiting."
    exit 0
fi

_log "started: workspace=$WORKSPACE pgid=$TARGET_PGID cycle=$CYCLE threshold=${THRESHOLD_S}s poll=${POLL_S}s warn_pct=${WARN_PCT}%"

# ── Compute derived thresholds ───────────────────────────────────────────────

WARN_S=$(( THRESHOLD_S * WARN_PCT / 100 ))

# ── Poll loop ────────────────────────────────────────────────────────────────

idle_clock_started=0   # set to 1 after first non-zero mtime observed
warn_emitted=0         # set to 1 once WARN has been logged for this crossing

# v9.4.0 — Phase-aware baseline (operator architectural request):
#   The watchdog used to compute idle = now - max(watched_mtime). That had two
#   defects: (a) stale mtimes from a prior run could fire instantly on resume
#   (startup-grace bug); (b) a phase that produces zero file writes (e.g. an
#   orchestrator turn that only reads files) appeared idle even when actively
#   progressing. Both are fixed by tracking the current phase and resetting a
#   PHASE_START_TIME each time cycle-state.json:phase changes. The effective
#   idle baseline becomes max(best_mtime, PHASE_START_TIME) so either signal —
#   a recent log write OR a recent phase advance — keeps the watchdog quiet.
START_TIME=$(date +%s)
LAST_PHASE=""
PHASE_START_TIME="$START_TIME"

while true; do

    # Gather newest mtime across workspace files and the cycle-state path.
    now=$(date +%s)
    best_mtime=0
    best_path=""

    # v9.4.0: detect phase transitions and reset the phase baseline.
    if [ -f "$CYCLE_STATE_PATH" ]; then
        current_phase=$(jq -r '.phase // empty' "$CYCLE_STATE_PATH" 2>/dev/null || echo "")
        if [ -n "$current_phase" ] && [ "$current_phase" != "$LAST_PHASE" ]; then
            if [ -n "$LAST_PHASE" ]; then
                _log "phase advance: '$LAST_PHASE' → '$current_phase' (resetting baseline to now)"
            else
                _log "phase observed: '$current_phase' (baseline anchored to start_time=$START_TIME)"
            fi
            PHASE_START_TIME="$now"
            LAST_PHASE="$current_phase"
            # Phase change → reset WARN flag too (a new phase deserves a fresh chance).
            warn_emitted=0
        fi
    fi

    # Scan workspace globs.
    for glob_pat in \
        "$WORKSPACE/*.log" \
        "$WORKSPACE/*.md" \
        "$WORKSPACE/*.json"; do
        result=$(_scan_glob "$glob_pat")
        m="${result%% *}"
        p="${result#* }"
        if [ -n "$m" ] && [ "$m" -gt "$best_mtime" ] 2>/dev/null; then
            best_mtime="$m"
            best_path="$p"
        fi
    done

    # Cycle-state path (may be outside workspace).
    if [ -f "$CYCLE_STATE_PATH" ]; then
        m=$(get_mtime "$CYCLE_STATE_PATH")
        if [ "$m" -gt "$best_mtime" ] 2>/dev/null; then
            best_mtime="$m"
            best_path="$CYCLE_STATE_PATH"
        fi
    fi

    # Ledger file (best-effort, if EVOLVE_PROJECT_ROOT is set).
    ledger_path="${EVOLVE_PROJECT_ROOT:-}/.evolve/ledger.jsonl"
    if [ -n "${EVOLVE_PROJECT_ROOT:-}" ] && [ -f "$ledger_path" ]; then
        m=$(get_mtime "$ledger_path")
        if [ "$m" -gt "$best_mtime" ] 2>/dev/null; then
            best_mtime="$m"
            best_path="$ledger_path"
        fi
    fi

    # Only start the idle clock once we have observed at least one real file.
    if [ "$best_mtime" -gt 0 ] 2>/dev/null; then
        idle_clock_started=1
    fi

    if [ "$idle_clock_started" = "1" ] && [ "$best_mtime" -gt 0 ] 2>/dev/null; then
        # v9.4.0: baseline is the LATER of (a) freshest watched-file mtime
        # and (b) the current phase's start time. Either signal — log activity
        # OR a recent phase advance — restarts the clock.
        baseline="$best_mtime"
        if [ "$PHASE_START_TIME" -gt "$baseline" ] 2>/dev/null; then
            baseline="$PHASE_START_TIME"
            best_path="<phase-start anchor (phase=$LAST_PHASE)>"
        fi
        idle_s=$(( now - baseline ))
        [ "$idle_s" -lt 0 ] && idle_s=0

        # WARN threshold crossing (emit only once per crossing).
        if [ "$idle_s" -ge "$WARN_S" ] && [ "$warn_emitted" = "0" ]; then
            _log "WARN: idle for ${idle_s}s (warn threshold ${WARN_S}s); stall threshold ${THRESHOLD_S}s; last activity: ${best_path}"
            warn_emitted=1
        fi

        # Reset WARN flag if activity resumed.
        if [ "$idle_s" -lt "$WARN_S" ]; then
            warn_emitted=0
        fi

        # FIRE condition.
        if [ "$idle_s" -ge "$THRESHOLD_S" ]; then
            _log "FIRE: idle for ${idle_s}s >= threshold ${THRESHOLD_S}s; last file: ${best_path}"

            # ── FIRE sequence ────────────────────────────────────────────────

            # a) Write stall-progress.json.
            checkpoint_ts=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
            stall_json="{\"idle_s\":${idle_s},\"threshold_s\":${THRESHOLD_S},\"last_mtime_file\":\"${best_path}\",\"checkpoint_ts\":\"${checkpoint_ts}\"}"
            stall_file="${WORKSPACE}/stall-progress.json"
            stall_tmp="${stall_file}.tmp.$$"
            printf '%s\n' "$stall_json" > "$stall_tmp" && mv -f "$stall_tmp" "$stall_file" || true
            _log "wrote stall-progress.json: $stall_file"
            _append_abnormal_event "$WORKSPACE" "idle_s=${idle_s} threshold_s=${THRESHOLD_S} last_file=${best_path} cycle=${CYCLE}"

            # b) Run checkpoint via cycle-state.sh (best-effort).
            __wdog_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
            cycle_state_sh="$__wdog_dir/../lifecycle/cycle-state.sh"
            if [ -f "$cycle_state_sh" ]; then
                EVOLVE_CYCLE_STATE_FILE="$CYCLE_STATE_PATH" \
                    bash "$cycle_state_sh" checkpoint stall-inactivity 2>/dev/null || true
                _log "checkpoint stall-inactivity requested (best-effort)"
            else
                _log "cycle-state.sh not found at $cycle_state_sh — skipping checkpoint"
            fi

            # c) Ignore SIGTERM on this watchdog process during kill sequence.
            trap '' TERM

            # d) Send SIGTERM to the target process group.
            _log "sending SIGTERM to pgid $TARGET_PGID"
            kill -TERM -"$TARGET_PGID" 2>/dev/null || true

            # e) Grace period.
            sleep "$GRACE_S"

            # f) Send SIGKILL for any survivors.
            _log "sending SIGKILL to pgid $TARGET_PGID (post-grace)"
            kill -KILL -"$TARGET_PGID" 2>/dev/null || true

            _log "kill sequence complete for pgid $TARGET_PGID"
            exit 0
        fi
    fi

    sleep "$POLL_S"
done
