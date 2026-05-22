#!/usr/bin/env bash
#
# show-trace.sh — Pretty-print the human-readable phase tracker trace.md.
#
# Auto-detects the active cycle from .evolve/cycle-state.json if --cycle is
# omitted. In --watch mode tails the file in real time. When trace.md does
# not yet exist (Phase A, before stream-json wiring), falls back to a
# synthesized summary built from existing *-stdout.log + *-usage.json.
#
# Usage:
#   bash scripts/observability/show-trace.sh                       # active cycle
#   bash scripts/observability/show-trace.sh --cycle=36
#   bash scripts/observability/show-trace.sh --cycle=36 --phase=scout
#   bash scripts/observability/show-trace.sh --cycle=36 --watch    # tail -F
#   bash scripts/observability/show-trace.sh --cycle=36 --summary  # synthesized fallback

set -uo pipefail

CYCLE=""
PHASE=""
WATCH=0
SUMMARY_ONLY=0

while [ $# -gt 0 ]; do
    case "$1" in
        --cycle=*) CYCLE="${1#*=}" ;;
        --phase=*) PHASE="${1#*=}" ;;
        --watch) WATCH=1 ;;
        --summary) SUMMARY_ONLY=1 ;;
        --help|-h) sed -n '2,20p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
        --*) echo "[show-trace] unknown flag: $1" >&2; exit 10 ;;
        *) echo "[show-trace] extra positional arg: $1" >&2; exit 10 ;;
    esac
    shift
done

PROJECT_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
RUNS_DIR="${RUNS_DIR_OVERRIDE:-$PROJECT_ROOT/.evolve/runs}"
CYCLE_STATE="$PROJECT_ROOT/.evolve/cycle-state.json"

# Auto-detect cycle.
if [ -z "$CYCLE" ]; then
    if [ -f "$CYCLE_STATE" ]; then
        CYCLE=$(jq -r '.cycle_id // empty' "$CYCLE_STATE" 2>/dev/null || true)
    fi
    if [ -z "$CYCLE" ]; then
        # Fall back to highest-numbered cycle dir.
        CYCLE=$(find "$RUNS_DIR" -maxdepth 1 -type d -name 'cycle-*' 2>/dev/null \
            | sed 's|.*/cycle-||' | sort -n | tail -1)
    fi
    [ -n "$CYCLE" ] || { echo "[show-trace] no active cycle detected; pass --cycle=N" >&2; exit 10; }
fi
[[ "$CYCLE" =~ ^[0-9]+$ ]] || { echo "[show-trace] cycle must be integer" >&2; exit 10; }

WORKSPACE="$RUNS_DIR/cycle-$CYCLE"
TRACE_FILE="$WORKSPACE/.ephemeral/trace.md"

# Synthesized fallback: build a one-line-per-phase summary from existing data.
print_summary() {
    local cycle=$1
    echo "Cycle $cycle synthesized summary (trace.md not yet available; uses *-timing.json + *-usage.json sidecars)"
    echo ""
    local found=0
    local cycle_state_phase=""
    local cycle_state_agent=""
    if [ -f "$CYCLE_STATE" ]; then
        local _cs_cycle
        _cs_cycle=$(jq -r '.cycle_id // empty' "$CYCLE_STATE" 2>/dev/null)
        if [ "$_cs_cycle" = "$cycle" ]; then
            cycle_state_phase=$(jq -r '.phase // empty' "$CYCLE_STATE" 2>/dev/null)
            cycle_state_agent=$(jq -r '.active_agent // empty' "$CYCLE_STATE" 2>/dev/null)
        fi
    fi
    printf "%-16s %-10s %-8s %-12s %-12s %s\n" "PHASE" "LATENCY" "TURNS" "COST" "MODEL(S)" "STATUS"
    printf "%-16s %-10s %-8s %-12s %-12s %s\n" "-----" "-------" "-----" "----" "--------" "------"
    while IFS= read -r t; do
        local p
        p=$(basename "$t" | sed 's/-timing\.json$//')
        if [ -n "$PHASE" ] && [ "$p" != "$PHASE" ]; then continue; fi
        local u="$WORKSPACE/${p}-usage.json"
        local lat_ms turns cost models
        lat_ms=$(jq -r '.total_ms // 0' "$t" 2>/dev/null || echo 0)
        if [ -f "$u" ]; then
            turns=$(jq -r '.num_turns // 0' "$u" 2>/dev/null || echo 0)
            cost=$(jq -r '.total_cost_usd // 0' "$u" 2>/dev/null || echo 0)
            models=$(jq -r '.modelUsage // {} | keys | join("+") | gsub("claude-"; "")' "$u" 2>/dev/null || echo "?")
        else
            turns="?"
            cost="?"
            models="?"
        fi
        local lat_h
        if [ "$lat_ms" -ge 60000 ]; then
            lat_h=$(awk -v m="$lat_ms" 'BEGIN { s=int(m/1000); printf "%dm%02ds", int(s/60), s%60 }')
        elif [ "$lat_ms" -ge 1000 ]; then
            lat_h=$(awk -v m="$lat_ms" 'BEGIN { printf "%.1fs", m/1000 }')
        else
            lat_h="${lat_ms}ms"
        fi
        local cost_fmt
        if [ "$cost" = "?" ]; then cost_fmt="?"; else cost_fmt=$(awk -v c="$cost" 'BEGIN { printf "$%.4f", c }'); fi
        local status="completed"
        if [ "$p" = "$cycle_state_agent" ] && [ -n "$cycle_state_phase" ]; then
            status="in-progress (phase=$cycle_state_phase)"
        fi
        printf "%-16s %-10s %-8s %-12s %-12s %s\n" "$p" "$lat_h" "$turns" "$cost_fmt" "$models" "$status"
        found=$((found + 1))
    done < <(find "$WORKSPACE" -maxdepth 1 -name '*-timing.json' -type f 2>/dev/null | sort)

    if [ "$found" -eq 0 ]; then
        echo "(no completed phases yet for cycle $cycle)"
        if [ -n "$cycle_state_agent" ]; then
            echo ""
            echo "In progress: agent=$cycle_state_agent phase=$cycle_state_phase"
        fi
    fi
}

if [ "$SUMMARY_ONLY" = "1" ] || [ ! -f "$TRACE_FILE" ]; then
    if [ ! -f "$TRACE_FILE" ] && [ "$WATCH" = "1" ]; then
        echo "[show-trace] no trace.md at $TRACE_FILE yet — Phase B wiring not active."
        echo "[show-trace] showing synthesized summary instead. Re-run with --watch when trace.md exists."
        echo ""
    fi
    print_summary "$CYCLE"
    exit 0
fi

if [ "$WATCH" = "1" ]; then
    echo "[show-trace] watching $TRACE_FILE (Ctrl-C to stop)"
    if [ -n "$PHASE" ]; then
        tail -F "$TRACE_FILE" 2>/dev/null | grep --line-buffered " $PHASE "
    else
        tail -F "$TRACE_FILE" 2>/dev/null
    fi
    exit 0
fi

if [ -n "$PHASE" ]; then
    grep " $PHASE " "$TRACE_FILE" || echo "(no events for phase=$PHASE in $TRACE_FILE)"
else
    cat "$TRACE_FILE"
fi
