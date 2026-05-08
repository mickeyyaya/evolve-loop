#!/usr/bin/env bash
#
# cycle-state.sh — Per-cycle runtime state helpers (v8.13.1).
#
# Manages .evolve/cycle-state.json, the runtime fact-of-truth that
# role-gate.sh and phase-gate-precondition.sh consult to make decisions.
#
# Schema (see PLAN.md):
#   {
#     "cycle_id": 8135,
#     "phase": "build",
#     "started_at": "2026-04-27T14:00:00Z",
#     "phase_started_at": "2026-04-27T14:05:00Z",
#     "active_agent": "builder",
#     "active_worktree": "/var/folders/.../evolve-build-cycle-8135-XXXX",
#     "completed_phases": ["calibrate","research","discover"],
#     "workspace_path": ".evolve/runs/cycle-8135"
#   }
#
# Lifecycle:
#   - cycle_state_init: writes the file at cycle start (phase=calibrate).
#   - cycle_state_advance: atomic update via mv of a temp file.
#   - cycle_state_clear: removes the file (cycle complete OR abort).
#   - cycle_state_get: prints a single field's value (used by hooks).
#
# Usage as library (sourced):
#   source scripts/lifecycle/cycle-state.sh
#   cycle_state_init 8135 .evolve/runs/cycle-8135
#   cycle_state_advance build builder /tmp/wt-foo
#   cycle_state_get phase
#   cycle_state_clear
#
# Usage as CLI (when invoked directly with subcommand):
#   bash scripts/lifecycle/cycle-state.sh init 8135 .evolve/runs/cycle-8135
#   bash scripts/lifecycle/cycle-state.sh advance build builder /tmp/wt-foo
#   bash scripts/lifecycle/cycle-state.sh get phase
#   bash scripts/lifecycle/cycle-state.sh clear
#   bash scripts/lifecycle/cycle-state.sh exists
#   bash scripts/lifecycle/cycle-state.sh dump

set -uo pipefail

# v8.18.0: dual-root resolution. cycle-state.json must be written to the user's
# project (writable), not to the plugin cache (read-only sensitive path under
# ~/.claude/). resolve-roots.sh defines EVOLVE_PROJECT_ROOT for writes.
__rr_self="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$__rr_self/resolve-roots.sh"
unset __rr_self

CYCLE_STATE_FILE="${EVOLVE_CYCLE_STATE_FILE:-$EVOLVE_PROJECT_ROOT/.evolve/cycle-state.json}"

_iso_now() {
    date -u +"%Y-%m-%dT%H:%M:%SZ"
}

cycle_state_path() {
    printf '%s\n' "$CYCLE_STATE_FILE"
}

cycle_state_exists() {
    [ -f "$CYCLE_STATE_FILE" ]
}

# Atomic write: write to <file>.tmp.$$, then mv.
_atomic_write() {
    local content="$1"
    local target="$CYCLE_STATE_FILE"
    local tmp="${target}.tmp.$$"
    mkdir -p "$(dirname "$target")"
    printf '%s\n' "$content" > "$tmp"
    mv -f "$tmp" "$target"
}

cycle_state_init() {
    local cycle_id="${1:?cycle_id required}"
    local workspace_path="${2:-.evolve/runs/cycle-${cycle_id}}"
    local now
    now=$(_iso_now)
    # v8.19.0: capture intent_required from env at init time so mid-stream
    # env-var flips don't break in-flight cycles. Cycles initialized without
    # the flag continue to operate under the default (no intent phase) flow
    # even if EVOLVE_REQUIRE_INTENT is later set.
    local intent_required="false"
    [ "${EVOLVE_REQUIRE_INTENT:-0}" = "1" ] && intent_required="true"
    if ! command -v jq >/dev/null 2>&1; then
        _atomic_write "{\"cycle_id\":${cycle_id},\"phase\":\"calibrate\",\"started_at\":\"${now}\",\"phase_started_at\":\"${now}\",\"active_agent\":null,\"active_worktree\":null,\"completed_phases\":[],\"workspace_path\":\"${workspace_path}\",\"intent_required\":${intent_required}}"
        return 0
    fi
    local json
    json=$(jq -nc \
        --argjson cycle_id "$cycle_id" \
        --arg now "$now" \
        --arg workspace "$workspace_path" \
        --argjson intent_required "$intent_required" \
        '{cycle_id: $cycle_id, phase: "calibrate", started_at: $now, phase_started_at: $now, active_agent: null, active_worktree: null, completed_phases: [], workspace_path: $workspace, intent_required: $intent_required}')
    _atomic_write "$json"
}

cycle_state_advance() {
    local new_phase="${1:?phase required}"
    local agent="${2:-}"
    local worktree="${3:-}"
    if ! cycle_state_exists; then
        echo "[cycle-state] ERROR: cannot advance — state file missing" >&2
        return 1
    fi
    if ! command -v jq >/dev/null 2>&1; then
        echo "[cycle-state] ERROR: jq required for advance" >&2
        return 1
    fi
    local now
    now=$(_iso_now)
    local current
    current=$(cat "$CYCLE_STATE_FILE")
    local agent_arg='null'
    [ -n "$agent" ] && agent_arg="\"$agent\""
    local worktree_arg='null'
    [ -n "$worktree" ] && worktree_arg="\"$worktree\""
    local updated
    updated=$(echo "$current" | jq -c \
        --arg new_phase "$new_phase" \
        --arg now "$now" \
        --argjson agent "$agent_arg" \
        --argjson worktree "$worktree_arg" \
        '
        . as $s
        | .phase as $cur
        | (["calibrate","intent","research","discover","plan-review","tdd","build","audit","ship","learn","retrospective"]) as $known
        | .completed_phases =
            (if ($known | index($cur)) and (($s.completed_phases | index($cur)) == null)
             then $s.completed_phases + [$cur]
             else $s.completed_phases end)
        | .phase = $new_phase
        | .phase_started_at = $now
        | .active_agent = (if $agent == null then .active_agent else $agent end)
        | .active_worktree = (if $worktree == null then .active_worktree else $worktree end)
        | del(.parallel_workers)
        ')
    _atomic_write "$updated"
}

cycle_state_set_parallel_workers() {
    # Sprint 1.1 observability: record that a fan-out dispatch is in flight.
    # Field shape: parallel_workers = {agent: <name>, count: <N>, started_at: <iso>}.
    # This is purely informational — phase-gate-precondition.sh continues to
    # gate on active_agent. dispatch-parallel writes one of these on entry,
    # clears it on exit.
    local agent="${1:?agent required}"
    local count="${2:?count required}"
    if ! cycle_state_exists; then
        echo "[cycle-state] ERROR: cannot set_parallel_workers — state file missing" >&2
        return 1
    fi
    if ! command -v jq >/dev/null 2>&1; then
        echo "[cycle-state] ERROR: jq required" >&2
        return 1
    fi
    local now; now=$(_iso_now)
    local current; current=$(cat "$CYCLE_STATE_FILE")
    local updated
    updated=$(echo "$current" | jq -c \
        --arg agent "$agent" \
        --argjson count "$count" \
        --arg now "$now" \
        '.parallel_workers = {agent: $agent, count: $count, started_at: $now}')
    _atomic_write "$updated"
}

cycle_state_clear_parallel_workers() {
    if ! cycle_state_exists; then return 0; fi
    if ! command -v jq >/dev/null 2>&1; then return 0; fi
    local current; current=$(cat "$CYCLE_STATE_FILE")
    local updated
    updated=$(echo "$current" | jq -c 'del(.parallel_workers)')
    _atomic_write "$updated"
}

# v8.23.0 Task D: extend parallel_workers with per-worker status tracking.
# Schema after this call: parallel_workers = {agent, count, started_at, workers: [{name, status, started_at, ended_at, exit_code}]}.
# Each worker initialized with status=pending, no started_at/ended_at/exit_code yet.
# Called by subagent-run.sh:cmd_dispatch_parallel BEFORE fanout-dispatch.sh spawns.
cycle_state_init_workers() {
    local agent="${1:?agent required}"
    shift
    if [ $# -lt 1 ]; then
        echo "[cycle-state] ERROR: init-workers requires at least one worker name" >&2
        return 1
    fi
    if ! cycle_state_exists; then
        echo "[cycle-state] ERROR: cannot init-workers — state file missing" >&2
        return 1
    fi
    if ! command -v jq >/dev/null 2>&1; then
        echo "[cycle-state] ERROR: jq required" >&2
        return 1
    fi
    # Build the workers JSON array via jq -n + --args (bash 3.2 safe).
    local workers_json
    workers_json=$(jq -nc --args '$ARGS.positional | map({name: ., status: "pending"})' -- "$@")
    local now; now=$(_iso_now)
    local count=$#
    local current; current=$(cat "$CYCLE_STATE_FILE")
    local updated
    updated=$(echo "$current" | jq -c \
        --arg agent "$agent" \
        --argjson count "$count" \
        --arg now "$now" \
        --argjson workers "$workers_json" \
        '.parallel_workers = {agent: $agent, count: $count, started_at: $now, workers: $workers}')
    _atomic_write "$updated"
}

# v8.23.0 Task D: atomic upsert of a worker's status into parallel_workers.workers[].
# Status values: pending | running | done | failed.
# When status is "running", started_at is recorded. When status is "done"/"failed",
# ended_at + exit_code are recorded. Caller passes exit_code for terminal statuses;
# omitted for "pending" / "running".
#
# Bash 3.2 safe — uses jq's map+if expressions, not associative arrays.
cycle_state_set_worker_status() {
    local name="${1:?worker name required}"
    local status="${2:?status required (pending|running|done|failed)}"
    local exit_code="${3:-}"
    case "$status" in
        pending|running|done|failed) ;;
        *) echo "[cycle-state] ERROR: invalid status '$status' (expected pending|running|done|failed)" >&2; return 1 ;;
    esac
    if ! cycle_state_exists; then
        echo "[cycle-state] ERROR: cannot set-worker-status — state file missing" >&2
        return 1
    fi
    if ! command -v jq >/dev/null 2>&1; then
        echo "[cycle-state] ERROR: jq required" >&2
        return 1
    fi
    local now; now=$(_iso_now)
    local current; current=$(cat "$CYCLE_STATE_FILE")
    # Build the worker patch as a jq filter that updates the matching entry, or
    # appends a new one if the worker name isn't already in the list.
    local exit_code_arg='null'
    [ -n "$exit_code" ] && exit_code_arg="$exit_code"
    local updated
    updated=$(echo "$current" | jq -c \
        --arg name "$name" \
        --arg status "$status" \
        --arg now "$now" \
        --argjson exit_code "$exit_code_arg" \
        '
        .parallel_workers = (.parallel_workers // {agent: "unknown", count: 0, started_at: $now, workers: []})
        | .parallel_workers.workers = (
            (.parallel_workers.workers // []) as $ws
            | (([range(0; ($ws | length)) | $ws[.] | select(.name == $name)] | length) > 0) as $exists
            | if $exists then
                $ws | map(
                    if .name == $name then
                        .status = $status
                        | (if $status == "running" then .started_at = (.started_at // $now) else . end)
                        | (if ($status == "done" or $status == "failed") then .ended_at = $now | .exit_code = $exit_code else . end)
                    else . end
                )
              else
                $ws + [{name: $name, status: $status} as $base
                       | (if $status == "running" then $base + {started_at: $now} else $base end)
                       | (if ($status == "done" or $status == "failed") then . + {ended_at: $now, exit_code: $exit_code} else . end)]
              end
          )
        ')
    _atomic_write "$updated"
}

cycle_state_set_agent() {
    local agent="${1:?agent required}"
    local worktree="${2:-}"
    if ! cycle_state_exists; then
        echo "[cycle-state] ERROR: cannot set_agent — state file missing" >&2
        return 1
    fi
    if ! command -v jq >/dev/null 2>&1; then
        echo "[cycle-state] ERROR: jq required" >&2
        return 1
    fi
    local current
    current=$(cat "$CYCLE_STATE_FILE")
    local worktree_arg='null'
    [ -n "$worktree" ] && worktree_arg="\"$worktree\""
    local updated
    updated=$(echo "$current" | jq -c \
        --arg agent "$agent" \
        --argjson worktree "$worktree_arg" \
        '.active_agent = $agent | .active_worktree = (if $worktree == null then .active_worktree else $worktree end)')
    _atomic_write "$updated"
}

# v8.22.0: auto-prune expired entries from state.json:failedApproaches[].
# Operates on EVOLVE_PROJECT_ROOT/.evolve/state.json (NOT cycle-state.json).
# Called by record-failure-to-state.sh and the dispatcher's record_failed_approach
# at write time so the file size stays bounded. failure-adapter.sh also calls
# this before computing its decision so stale entries can't poison the lookback.
#
# An entry is "expired" if its expiresAt timestamp (ISO-8601) is older than now.
# Entries without expiresAt (legacy) are kept (no false-prune of pre-v8.22 data).
cycle_state_prune_expired_failures() {
    local state_file="${1:-$EVOLVE_PROJECT_ROOT/.evolve/state.json}"
    [ -f "$state_file" ] || return 0
    if ! command -v jq >/dev/null 2>&1; then
        echo "[cycle-state] WARN: jq missing; cannot prune-expired" >&2
        return 0
    fi
    local now_s; now_s=$(date -u +%s)
    local before; before=$(jq '(.failedApproaches // []) | length' "$state_file")
    local tmp="${state_file}.tmp.$$"
    # v8.23.1: same legacy-handling logic as failure-adapter.sh's read filter.
    # Entries with null expiresAt + non-null recordedAt get an effective TTL
    # of recordedAt + 1d (matches the tightest classification age-out window)
    # so legacy entries age out instead of poisoning the lookback indefinitely.
    jq --argjson now "$now_s" \
        '.failedApproaches = ((.failedApproaches // []) | map(
            select(
                ((.expiresAt // null) != null and ((.expiresAt | (try fromdateiso8601 catch ($now + 1))) > $now))
                or ((.expiresAt // null) == null and (.recordedAt // null) == null)
                or ((.expiresAt // null) == null and (.recordedAt // null) != null
                    and ((.recordedAt | (try fromdateiso8601 catch 0)) + 86400) > $now)
            )
        ))' "$state_file" > "$tmp" && mv -f "$tmp" "$state_file"
    local after; after=$(jq '(.failedApproaches // []) | length' "$state_file")
    local removed=$((before - after))
    if [ "$removed" -gt 0 ]; then
        echo "[cycle-state] prune-expired: removed $removed expired failedApproaches entries (before=$before after=$after)" >&2
    fi
    return 0
}

# v8.21.0: privileged-shell sets active_worktree without changing phase.
# Called by run-cycle.sh after `git worktree add` succeeds. The orchestrator
# is denied this command at the profile level — only privileged shell context
# may write the canonical worktree path.
cycle_state_set_worktree() {
    local worktree="${1:?worktree path required}"
    if ! cycle_state_exists; then
        echo "[cycle-state] ERROR: cannot set-worktree — state file missing" >&2
        return 1
    fi
    if ! command -v jq >/dev/null 2>&1; then
        echo "[cycle-state] ERROR: jq required for set-worktree" >&2
        return 1
    fi
    local current
    current=$(cat "$CYCLE_STATE_FILE")
    local updated
    updated=$(echo "$current" | jq -c --arg wt "$worktree" '.active_worktree = $wt')
    _atomic_write "$updated"
}

cycle_state_clear() {
    if [ -f "$CYCLE_STATE_FILE" ]; then
        rm -f "$CYCLE_STATE_FILE"
    fi
}

cycle_state_get() {
    local field="${1:?field required}"
    if ! cycle_state_exists; then
        return 1
    fi
    if command -v jq >/dev/null 2>&1; then
        jq -r ".${field} // empty" "$CYCLE_STATE_FILE"
    else
        # Naive sed fallback for top-level scalar fields.
        sed -n "s/.*\"${field}\"[[:space:]]*:[[:space:]]*\"\\([^\"]*\\)\".*/\\1/p" "$CYCLE_STATE_FILE" | head -1
    fi
}

cycle_state_dump() {
    if ! cycle_state_exists; then
        echo "(no cycle-state.json)" >&2
        return 1
    fi
    cat "$CYCLE_STATE_FILE"
}

# CLI dispatcher: only fires when this file is executed directly.
if [ "${BASH_SOURCE[0]}" = "${0}" ]; then
    cmd="${1:-}"
    shift || true
    case "$cmd" in
        init)                    cycle_state_init "$@" ;;
        advance)                 cycle_state_advance "$@" ;;
        set-agent)               cycle_state_set_agent "$@" ;;
        set-worktree)            cycle_state_set_worktree "$@" ;;
        prune-expired-failures)  cycle_state_prune_expired_failures "$@" ;;
        set-parallel-workers)    cycle_state_set_parallel_workers "$@" ;;
        clear-parallel-workers)  cycle_state_clear_parallel_workers ;;
        init-workers)            cycle_state_init_workers "$@" ;;
        set-worker-status)       cycle_state_set_worker_status "$@" ;;
        clear)                   cycle_state_clear ;;
        get)                     cycle_state_get "$@" ;;
        exists)                  cycle_state_exists && echo yes || { echo no; exit 1; } ;;
        dump)                    cycle_state_dump ;;
        path)                    cycle_state_path ;;
        *)                       echo "usage: cycle-state.sh {init|advance|set-agent|set-worktree|set-parallel-workers|clear-parallel-workers|init-workers|set-worker-status|prune-expired-failures|clear|get|exists|dump|path}" >&2; exit 2 ;;
    esac
fi
