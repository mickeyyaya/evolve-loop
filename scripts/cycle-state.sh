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
#   source scripts/cycle-state.sh
#   cycle_state_init 8135 .evolve/runs/cycle-8135
#   cycle_state_advance build builder /tmp/wt-foo
#   cycle_state_get phase
#   cycle_state_clear
#
# Usage as CLI (when invoked directly with subcommand):
#   bash scripts/cycle-state.sh init 8135 .evolve/runs/cycle-8135
#   bash scripts/cycle-state.sh advance build builder /tmp/wt-foo
#   bash scripts/cycle-state.sh get phase
#   bash scripts/cycle-state.sh clear
#   bash scripts/cycle-state.sh exists
#   bash scripts/cycle-state.sh dump

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CYCLE_STATE_FILE="${EVOLVE_CYCLE_STATE_FILE:-$REPO_ROOT/.evolve/cycle-state.json}"

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
    if ! command -v jq >/dev/null 2>&1; then
        # Fallback: hand-rolled JSON if jq not available (rare on dev machines).
        _atomic_write "{\"cycle_id\":${cycle_id},\"phase\":\"calibrate\",\"started_at\":\"${now}\",\"phase_started_at\":\"${now}\",\"active_agent\":null,\"active_worktree\":null,\"completed_phases\":[],\"workspace_path\":\"${workspace_path}\"}"
        return 0
    fi
    local json
    json=$(jq -nc \
        --argjson cycle_id "$cycle_id" \
        --arg now "$now" \
        --arg workspace "$workspace_path" \
        '{cycle_id: $cycle_id, phase: "calibrate", started_at: $now, phase_started_at: $now, active_agent: null, active_worktree: null, completed_phases: [], workspace_path: $workspace}')
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
        | (["calibrate","research","discover","tdd","build","audit","ship","learn"]) as $known
        | .completed_phases =
            (if ($known | index($cur)) and (($s.completed_phases | index($cur)) == null)
             then $s.completed_phases + [$cur]
             else $s.completed_phases end)
        | .phase = $new_phase
        | .phase_started_at = $now
        | .active_agent = (if $agent == null then .active_agent else $agent end)
        | .active_worktree = (if $worktree == null then .active_worktree else $worktree end)
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
        init)         cycle_state_init "$@" ;;
        advance)      cycle_state_advance "$@" ;;
        set-agent)    cycle_state_set_agent "$@" ;;
        clear)        cycle_state_clear ;;
        get)          cycle_state_get "$@" ;;
        exists)       cycle_state_exists && echo yes || { echo no; exit 1; } ;;
        dump)         cycle_state_dump ;;
        path)         cycle_state_path ;;
        *)            echo "usage: cycle-state.sh {init|advance|set-agent|clear|get|exists|dump|path}" >&2; exit 2 ;;
    esac
fi
