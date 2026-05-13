#!/usr/bin/env bash
#
# dashboard.sh — Combined one-glance observability view for the active or
# specified cycle. Wraps:
#
#   - show-cycle-cost.sh         (per-phase cost + token breakdown)
#   - cat ./.ephemeral/metrics/cycle-metrics.json  (rollup, if present)
#   - tail trace.md              (most recent N tracker events)
#   - cycle-state.json           (current phase + worktree)
#
# Designed for human-tail use during long /evolve-loop runs and for
# operator review post-cycle. Does NOT mutate state.
#
# Usage:
#   bash scripts/observability/dashboard.sh                  # active cycle
#   bash scripts/observability/dashboard.sh --cycle=42
#   bash scripts/observability/dashboard.sh --watch          # refresh every 5s
#   bash scripts/observability/dashboard.sh --watch=2        # refresh every 2s
#   bash scripts/observability/dashboard.sh --json           # machine-readable summary
#   bash scripts/observability/dashboard.sh --trace-lines=40 # tail N lines (default 20)
#
# Exit codes:
#   0  — at least one signal rendered
#   1  — no cycle workspace found
#  10  — bad arguments

set -uo pipefail

CYCLE=""
WATCH=0
WATCH_INTERVAL=5
JSON=0
TRACE_LINES=20

while [ $# -gt 0 ]; do
    case "$1" in
        --cycle=*) CYCLE="${1#*=}" ;;
        --watch) WATCH=1 ;;
        --watch=*) WATCH=1; WATCH_INTERVAL="${1#*=}" ;;
        --json) JSON=1 ;;
        --trace-lines=*) TRACE_LINES="${1#*=}" ;;
        --help|-h) sed -n '2,25p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
        --*) echo "[dashboard] unknown flag: $1" >&2; exit 10 ;;
        *) echo "[dashboard] unexpected positional: $1" >&2; exit 10 ;;
    esac
    shift
done

[[ "$WATCH_INTERVAL" =~ ^[0-9]+$ ]] || { echo "[dashboard] --watch interval must be integer" >&2; exit 10; }
[[ "$TRACE_LINES" =~ ^[0-9]+$ ]] || { echo "[dashboard] --trace-lines must be integer" >&2; exit 10; }

PROJECT_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
RUNS_DIR="${RUNS_DIR_OVERRIDE:-$PROJECT_ROOT/.evolve/runs}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# ---- Resolve active cycle if --cycle omitted -----------------------------
resolve_active_cycle() {
    local state="$PROJECT_ROOT/.evolve/cycle-state.json"
    if [ -f "$state" ] && command -v jq >/dev/null 2>&1; then
        jq -r '.cycle_id // empty' "$state" 2>/dev/null
    fi
}

if [ -z "$CYCLE" ]; then
    CYCLE=$(resolve_active_cycle)
    if [ -z "$CYCLE" ]; then
        # Fall back to highest-numbered cycle dir.
        CYCLE=$(ls -1 "$RUNS_DIR" 2>/dev/null \
            | grep -E '^cycle-[0-9]+$' \
            | sed 's/^cycle-//' \
            | sort -n | tail -1)
    fi
fi

[ -n "$CYCLE" ] || { echo "[dashboard] no active cycle found and --cycle not specified" >&2; exit 1; }
[[ "$CYCLE" =~ ^[0-9]+$ ]] || { echo "[dashboard] cycle must be integer: $CYCLE" >&2; exit 10; }

WORKSPACE="$RUNS_DIR/cycle-$CYCLE"
[ -d "$WORKSPACE" ] || { echo "[dashboard] no workspace at $WORKSPACE" >&2; exit 1; }

# ---- JSON summary mode ---------------------------------------------------
emit_json() {
    local state="$PROJECT_ROOT/.evolve/cycle-state.json"
    local phase="unknown"
    local last_role=""
    local last_exit=""
    if [ -f "$state" ] && command -v jq >/dev/null 2>&1; then
        phase=$(jq -r '.current_phase // "unknown"' "$state" 2>/dev/null)
    fi

    local metrics_file="$WORKSPACE/.ephemeral/metrics/cycle-metrics.json"
    local cost_json="{}"
    if [ -x "$SCRIPT_DIR/show-cycle-cost.sh" ]; then
        cost_json=$(bash "$SCRIPT_DIR/show-cycle-cost.sh" "$CYCLE" --json 2>/dev/null || echo '{}')
    fi

    local artifacts
    artifacts=$(ls -1 "$WORKSPACE"/*-report.md 2>/dev/null \
        | sed 's|.*/||' | tr '\n' ',' | sed 's/,$//')

    jq -n \
        --argjson cycle "$CYCLE" \
        --arg phase "$phase" \
        --arg workspace "$WORKSPACE" \
        --arg artifacts "$artifacts" \
        --argjson cost "$cost_json" \
        --arg has_metrics "$([ -f "$metrics_file" ] && echo yes || echo no)" \
        --arg has_trace "$([ -f "$WORKSPACE/.ephemeral/trace.md" ] && echo yes || echo no)" \
        '{
            cycle: $cycle,
            phase: $phase,
            workspace: $workspace,
            artifacts: ($artifacts | split(",") | map(select(length > 0))),
            cost: $cost,
            tracker: { has_metrics: $has_metrics, has_trace: $has_trace }
        }'
}

# ---- Human-readable render -----------------------------------------------
render_section() {
    local title="$1"
    local body="$2"
    [ -z "$body" ] && return
    echo ""
    echo "─── $title ───"
    echo "$body"
}

render_human() {
    clear 2>/dev/null || true
    echo "╭───────────────────────────────────────────────────────────────╮"
    printf "│ evolve-loop dashboard — cycle %-31s│\n" "$CYCLE"
    echo "├───────────────────────────────────────────────────────────────┤"

    # Active phase + state
    local state="$PROJECT_ROOT/.evolve/cycle-state.json"
    if [ -f "$state" ] && command -v jq >/dev/null 2>&1; then
        local phase ts worktree
        phase=$(jq -r '.current_phase // "—"' "$state" 2>/dev/null)
        ts=$(jq -r '.last_updated // "—"' "$state" 2>/dev/null)
        worktree=$(jq -r '.active_worktree // "—"' "$state" 2>/dev/null)
        printf "│ phase:      %-50s│\n" "$phase"
        printf "│ last seen:  %-50s│\n" "$ts"
        printf "│ worktree:   %-50s│\n" "${worktree:0:50}"
    fi
    printf "│ workspace:  %-50s│\n" "${WORKSPACE:0:50}"
    echo "╰───────────────────────────────────────────────────────────────╯"

    # Cost table (always)
    if [ -x "$SCRIPT_DIR/show-cycle-cost.sh" ]; then
        render_section "Cost & tokens (show-cycle-cost.sh)" \
            "$(bash "$SCRIPT_DIR/show-cycle-cost.sh" "$CYCLE" 2>/dev/null)"
    fi

    # Cycle-metrics rollup if present
    local metrics_file="$WORKSPACE/.ephemeral/metrics/cycle-metrics.json"
    if [ -f "$metrics_file" ] && command -v jq >/dev/null 2>&1; then
        local hot
        hot=$(jq -r '.hot_spots[]? // empty' "$metrics_file" 2>/dev/null | head -5)
        local total_wall total_cost
        total_wall=$(jq -r '.total_wall_ms // 0' "$metrics_file" 2>/dev/null)
        total_cost=$(jq -r '.total_cost_usd // 0' "$metrics_file" 2>/dev/null)
        local pretty="total wall: $((total_wall / 1000))s · total cost: \$${total_cost}"
        if [ -n "$hot" ]; then
            pretty+=$'\nhot spots:\n'"$hot"
        fi
        render_section "Cycle metrics rollup (.ephemeral/metrics/)" "$pretty"
    else
        render_section "Cycle metrics rollup" "(not yet — set EVOLVE_TRACKER_ENABLED=1 to enable)"
    fi

    # trace.md tail
    local trace_file="$WORKSPACE/.ephemeral/trace.md"
    if [ -f "$trace_file" ]; then
        render_section "trace.md (last $TRACE_LINES events)" \
            "$(tail -n "$TRACE_LINES" "$trace_file")"
    fi

    # Phase report headlines
    local reports
    reports=$(ls -1 "$WORKSPACE"/*-report.md 2>/dev/null)
    if [ -n "$reports" ]; then
        local headline
        headline=$(while IFS= read -r r; do
            [ -z "$r" ] && continue
            local first
            first=$(head -1 "$r" 2>/dev/null | sed 's/^# *//')
            printf "  %-40s  %s\n" "$(basename "$r")" "${first:0:60}"
        done <<< "$reports")
        render_section "Phase reports" "$headline"
    fi

    echo ""
}

# ---- Dispatch ------------------------------------------------------------
if [ "$JSON" = "1" ]; then
    emit_json
    exit 0
fi

if [ "$WATCH" = "1" ]; then
    while true; do
        render_human
        sleep "$WATCH_INTERVAL" 2>/dev/null || sleep 5
    done
else
    render_human
fi
