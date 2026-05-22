#!/usr/bin/env bash
#
# show-context-monitor.sh — Operator-facing live telemetry for cycle context
# usage (v9.1.0 Cycle 6).
#
# WHY THIS EXISTS
#
# Pre-v9.1.0, per-phase prompt size was visible only in a stderr WARN line
# that scrolled past during a multi-cycle dispatch. Operators had no way
# to ask "how much context has this cycle burned so far?" without grepping
# logs. v9.1.0 Cycle 6 writes context-monitor.json per phase invocation;
# this script tabulates the data for human consumption.
#
# Usage:
#   bash scripts/observability/show-context-monitor.sh <cycle>
#       Tabulate per-phase input tokens for the given cycle.
#
#   bash scripts/observability/show-context-monitor.sh --watch
#       Live-tail the most recent cycle (re-reads context-monitor.json
#       every 3 seconds).
#
#   bash scripts/observability/show-context-monitor.sh --json <cycle>
#       Emit the raw JSON for scripting.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$SCRIPT_DIR/../lifecycle/resolve-roots.sh"

usage() {
    sed -n '2,24p' "$0" | sed 's/^# \{0,1\}//'
}

if [ $# -eq 0 ] || [ "$1" = "--help" ] || [ "$1" = "-h" ]; then
    usage
    exit 0
fi

WATCH_MODE=0
JSON_MODE=0
CYCLE=""

while [ $# -gt 0 ]; do
    case "$1" in
        --watch) WATCH_MODE=1; shift ;;
        --json)  JSON_MODE=1; shift ;;
        --*)     echo "unknown flag: $1" >&2; exit 1 ;;
        *)       CYCLE="$1"; shift ;;
    esac
done

# Locate the runs dir.
RUNS_DIR="${RUNS_DIR_OVERRIDE:-$EVOLVE_PROJECT_ROOT/.evolve/runs}"

resolve_latest_cycle() {
    # Find the cycle dir with the most recent context-monitor.json.
    local newest=""
    local newest_ts=0
    for d in "$RUNS_DIR"/cycle-*/; do
        [ -d "$d" ] || continue
        local mon="$d/context-monitor.json"
        [ -f "$mon" ] || continue
        local ts
        ts=$(stat -f '%m' "$mon" 2>/dev/null || stat -c '%Y' "$mon" 2>/dev/null || echo 0)
        if [ "$ts" -gt "$newest_ts" ]; then
            newest_ts=$ts
            newest="$d"
        fi
    done
    [ -n "$newest" ] && basename "$newest" | sed 's/cycle-//'
}

render() {
    local cycle="$1"
    local mon="$RUNS_DIR/cycle-$cycle/context-monitor.json"
    if [ ! -f "$mon" ]; then
        echo "no context-monitor.json found at $mon" >&2
        return 1
    fi
    if [ "$JSON_MODE" = "1" ]; then
        cat "$mon"
        return 0
    fi
    if ! command -v jq >/dev/null 2>&1; then
        echo "jq required for tabular rendering — fallback to --json" >&2
        cat "$mon"
        return 0
    fi
    echo "Cycle $cycle context-monitor:"
    echo "  last updated: $(jq -r '.lastUpdated // "?"' "$mon")"
    echo ""
    printf "  %-20s %12s %12s %8s\n" "phase" "input_tokens" "cap_tokens" "cap_pct"
    printf "  %-20s %12s %12s %8s\n" "--------------------" "------------" "------------" "--------"
    jq -r '.phases | to_entries | sort_by(.value.measuredAt // "") | .[] |
            "  \(.key | tostring | (.+("                    ") | .[0:20]))  " +
            "\(.value.input_tokens | tostring | (.+("            ") | .[0:12]))  " +
            "\(.value.cap_tokens | tostring | (.+("            ") | .[0:12]))  " +
            "\(.value.cap_pct | tostring | (.+("        ") | .[0:8]))"' "$mon"
    echo ""
    local cum_tokens cum_cap cum_pct
    cum_tokens=$(jq -r '.cumulative_input_tokens // 0' "$mon")
    cum_cap=$(jq -r '.cumulative_cap // 0' "$mon")
    cum_pct=$(jq -r '.cumulative_pct // 0' "$mon")
    echo "  CUMULATIVE: $cum_tokens / $cum_cap tokens ($cum_pct%)"
    # WARN/CRITICAL flags
    local warn_pct="${EVOLVE_CHECKPOINT_WARN_AT_PCT:-80}"
    local crit_pct="${EVOLVE_CHECKPOINT_AT_PCT:-95}"
    if [ "$cum_pct" -ge "$crit_pct" ]; then
        echo "  >>> CRITICAL: cumulative >= ${crit_pct}% — next phase will signal checkpoint"
    elif [ "$cum_pct" -ge "$warn_pct" ]; then
        echo "  >>> WARN: cumulative >= ${warn_pct}% — approaching checkpoint threshold"
    fi
}

if [ "$WATCH_MODE" = "1" ]; then
    while true; do
        clear 2>/dev/null || true
        latest=$(resolve_latest_cycle)
        if [ -z "$latest" ]; then
            echo "no context-monitor.json in any cycle dir under $RUNS_DIR"
        else
            render "$latest"
        fi
        echo ""
        echo "  (refresh every 3s; Ctrl-C to exit)"
        sleep 3
    done
else
    if [ -z "$CYCLE" ]; then
        CYCLE=$(resolve_latest_cycle)
        [ -n "$CYCLE" ] || { echo "no cycles found and no <cycle> argument given" >&2; exit 1; }
        echo "(no cycle specified; showing latest = $CYCLE)" >&2
    fi
    render "$CYCLE"
fi
