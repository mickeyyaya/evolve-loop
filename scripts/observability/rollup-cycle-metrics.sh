#!/usr/bin/env bash
#
# rollup-cycle-metrics.sh — Cycle-level metrics aggregator.
#
# Reads all <agent>-timing.json + <agent>-usage.json sidecars in a cycle
# workspace and produces .evolve/runs/cycle-N/.ephemeral/metrics/cycle-metrics.json
# with per-phase rows, totals, model split, cache hit rate, and hot-spot
# detection.
#
# Layered on top of subagent-run.sh's existing sidecar capture; no new live
# instrumentation. Safe to run retroactively on any historical cycle.
#
# Usage:
#   bash scripts/observability/rollup-cycle-metrics.sh <cycle>
#   bash scripts/observability/rollup-cycle-metrics.sh <cycle> --stdout      # print to stdout instead of writing file
#   bash scripts/observability/rollup-cycle-metrics.sh <cycle> --baseline=5  # include last-N-cycles baseline (default 0)
#
# Exit codes:
#   0 — rollup produced
#   1 — workspace missing
#  10 — bad arguments

set -uo pipefail

CYCLE=""
STDOUT=0
BASELINE=0

while [ $# -gt 0 ]; do
    case "$1" in
        --stdout) STDOUT=1 ;;
        --baseline=*) BASELINE="${1#*=}" ;;
        --help|-h) sed -n '2,25p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
        --*) echo "[rollup-cycle-metrics] unknown flag: $1" >&2; exit 10 ;;
        *)
            if [ -z "$CYCLE" ]; then CYCLE="$1"
            else echo "[rollup-cycle-metrics] extra positional arg: $1" >&2; exit 10
            fi ;;
    esac
    shift
done

[ -n "$CYCLE" ] || { echo "[rollup-cycle-metrics] usage: rollup-cycle-metrics.sh <cycle> [--stdout] [--baseline=N]" >&2; exit 10; }
[[ "$CYCLE" =~ ^[0-9]+$ ]] || { echo "[rollup-cycle-metrics] cycle must be integer" >&2; exit 10; }
[[ "$BASELINE" =~ ^[0-9]+$ ]] || { echo "[rollup-cycle-metrics] --baseline must be integer" >&2; exit 10; }

command -v jq >/dev/null 2>&1 || { echo "[rollup-cycle-metrics] jq required" >&2; exit 1; }

# Resolve project root the same way show-cycle-cost.sh does.
PROJECT_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
RUNS_DIR="${RUNS_DIR_OVERRIDE:-$PROJECT_ROOT/.evolve/runs}"
WORKSPACE="$RUNS_DIR/cycle-$CYCLE"
[ -d "$WORKSPACE" ] || { echo "[rollup-cycle-metrics] no workspace at $WORKSPACE" >&2; exit 1; }

# Discover phases by enumerating *-timing.json sidecars (canonical phase list).
# Bash 3.2: no mapfile; use while-read into indexed array.
PHASES=()
while IFS= read -r f; do
    phase=$(basename "$f" | sed 's/-timing\.json$//')
    PHASES+=("$phase")
done < <(find "$WORKSPACE" -maxdepth 1 -name '*-timing.json' -type f 2>/dev/null | sort)

[ "${#PHASES[@]}" -gt 0 ] || { echo "[rollup-cycle-metrics] no *-timing.json sidecars in $WORKSPACE" >&2; exit 1; }

# Build per-phase JSON rows.
ROWS_TMP=$(mktemp -t rollup-rows.XXXXXX)
trap 'rm -f "$ROWS_TMP"' EXIT

TOTAL_MS=0
TOTAL_COST="0"
TOTAL_CACHE_READ=0
TOTAL_CACHE_CREATION=0
MODELS_SEEN_TMP=$(mktemp -t rollup-models.XXXXXX)
trap 'rm -f "$ROWS_TMP" "$MODELS_SEEN_TMP"' EXIT

for phase in "${PHASES[@]}"; do
    timing="$WORKSPACE/${phase}-timing.json"
    usage="$WORKSPACE/${phase}-usage.json"
    [ -f "$timing" ] || continue

    latency_ms=$(jq -r '.total_ms // 0' "$timing" 2>/dev/null || echo 0)
    cost_usd="0"
    turns=0
    cache_read=0
    cache_creation=0
    if [ -f "$usage" ]; then
        cost_usd=$(jq -r '.total_cost_usd // 0' "$usage" 2>/dev/null || echo 0)
        turns=$(jq -r '.num_turns // 0' "$usage" 2>/dev/null || echo 0)
        cache_read=$(jq -r '.usage.cache_read_input_tokens // 0' "$usage" 2>/dev/null || echo 0)
        cache_creation=$(jq -r '.usage.cache_creation_input_tokens // 0' "$usage" 2>/dev/null || echo 0)
        # Per-model entries.
        jq -r '.modelUsage // {} | to_entries[] | .key' "$usage" 2>/dev/null >> "$MODELS_SEEN_TMP" || true
    fi

    TOTAL_MS=$((TOTAL_MS + latency_ms))
    TOTAL_COST=$(awk -v a="$TOTAL_COST" -v b="$cost_usd" 'BEGIN { printf "%.6f", a + b }')
    TOTAL_CACHE_READ=$((TOTAL_CACHE_READ + cache_read))
    TOTAL_CACHE_CREATION=$((TOTAL_CACHE_CREATION + cache_creation))

    jq -nc \
        --arg phase "$phase" \
        --argjson latency_ms "$latency_ms" \
        --argjson cost_usd "$cost_usd" \
        --argjson turns "$turns" \
        --argjson cache_read "$cache_read" \
        --argjson cache_creation "$cache_creation" \
        '{phase: $phase, latency_ms: $latency_ms, cost_usd: $cost_usd, turns: $turns, cache_read_tokens: $cache_read, cache_creation_tokens: $cache_creation}' \
        >> "$ROWS_TMP"
done

# Models seen (deduped, sorted).
MODELS_JSON=$(sort -u "$MODELS_SEEN_TMP" | jq -R . | jq -s 'map(select(length>0))')

# Cache hit rate = cache_read / (cache_read + cache_creation). Guard against /0.
TOTAL_CACHE_DENOM=$((TOTAL_CACHE_READ + TOTAL_CACHE_CREATION))
if [ "$TOTAL_CACHE_DENOM" -gt 0 ]; then
    CACHE_HIT_RATE=$(awk -v r="$TOTAL_CACHE_READ" -v d="$TOTAL_CACHE_DENOM" 'BEGIN { printf "%.4f", r / d }')
else
    CACHE_HIT_RATE="0"
fi

# Hot-spot detection: phases that consume >40% of total wall time.
HOT_SPOTS=$(jq -nc \
    --slurpfile rows "$ROWS_TMP" \
    --argjson total_ms "$TOTAL_MS" \
    '$rows | map(select(.latency_ms * 100 / (if $total_ms == 0 then 1 else $total_ms end) > 40))
          | map(.phase + ": " + (.latency_ms | tostring) + "ms (" + ((.latency_ms * 100 / (if $total_ms == 0 then 1 else $total_ms end)) | floor | tostring) + "% of cycle)")')

# Phase list as a JSON array.
PHASES_JSON=$(jq -s '.' "$ROWS_TMP")

# Wall-clock from earliest timing → latest usage (approximate; sidecars don't
# carry absolute timestamps, so we use file mtimes — bash 3.2 portable).
get_mtime() {
    # POSIX-portable mtime in epoch seconds.
    if stat -f "%m" "$1" >/dev/null 2>&1; then
        stat -f "%m" "$1"   # BSD / macOS
    else
        stat -c "%Y" "$1"   # GNU / Linux
    fi
}

EARLIEST=""
LATEST=""
for phase in "${PHASES[@]}"; do
    timing="$WORKSPACE/${phase}-timing.json"
    [ -f "$timing" ] || continue
    mt=$(get_mtime "$timing")
    if [ -z "$EARLIEST" ] || [ "$mt" -lt "$EARLIEST" ]; then EARLIEST="$mt"; fi
    if [ -z "$LATEST" ] || [ "$mt" -gt "$LATEST" ]; then LATEST="$mt"; fi
done

iso_from_epoch() {
    if date -u -r "$1" -Iseconds >/dev/null 2>&1; then
        date -u -r "$1" -Iseconds
    elif command -v gdate >/dev/null 2>&1; then
        gdate -u -d "@$1" -Iseconds
    else
        date -u -r "$1" "+%Y-%m-%dT%H:%M:%SZ"
    fi
}

WALL_START=$(iso_from_epoch "$EARLIEST")
WALL_END=$(iso_from_epoch "$LATEST")

# Assemble the rollup.
ROLLUP=$(jq -nc \
    --argjson cycle "$CYCLE" \
    --arg wall_start "$WALL_START" \
    --arg wall_end "$WALL_END" \
    --argjson total_ms "$TOTAL_MS" \
    --argjson total_cost "$TOTAL_COST" \
    --argjson phases "$PHASES_JSON" \
    --argjson models "$MODELS_JSON" \
    --argjson cache_hit_rate "$CACHE_HIT_RATE" \
    --argjson hot_spots "$HOT_SPOTS" \
    '{
      schema_version: "1.0",
      cycle: $cycle,
      wall_clock_start: $wall_start,
      wall_clock_end: $wall_end,
      total_wall_ms: $total_ms,
      total_cost_usd: $total_cost,
      phases: $phases,
      models_used: $models,
      cache_hit_rate: $cache_hit_rate,
      hot_spots: $hot_spots
    }')

# Baseline comparison (optional).
if [ "$BASELINE" -gt 0 ]; then
    BASELINE_ROWS=$(mktemp -t rollup-base.XXXXXX)
    trap 'rm -f "$ROWS_TMP" "$MODELS_SEEN_TMP" "$BASELINE_ROWS"' EXIT
    # Find the previous N cycles with cycle-metrics.json files.
    prev_cycle=$((CYCLE - 1))
    found=0
    while [ "$prev_cycle" -gt 0 ] && [ "$found" -lt "$BASELINE" ]; do
        prev_metrics="$RUNS_DIR/cycle-$prev_cycle/.ephemeral/metrics/cycle-metrics.json"
        if [ -f "$prev_metrics" ]; then
            jq -c '{cycle: .cycle, total_wall_ms: .total_wall_ms, total_cost_usd: .total_cost_usd}' "$prev_metrics" 2>/dev/null >> "$BASELINE_ROWS" || true
            found=$((found + 1))
        fi
        prev_cycle=$((prev_cycle - 1))
    done
    if [ -s "$BASELINE_ROWS" ]; then
        BASELINE_JSON=$(jq -s '.' "$BASELINE_ROWS")
        ROLLUP=$(echo "$ROLLUP" | jq --argjson b "$BASELINE_JSON" '. + {baseline_cycles: $b}')
    fi
fi

if [ "$STDOUT" = "1" ]; then
    echo "$ROLLUP" | jq '.'
else
    OUT_DIR="$WORKSPACE/.ephemeral/metrics"
    OUT_FILE="$OUT_DIR/cycle-metrics.json"
    mkdir -p "$OUT_DIR"
    # Atomic write via mv-of-temp.
    TMP_OUT="$OUT_FILE.tmp.$$"
    echo "$ROLLUP" | jq '.' > "$TMP_OUT"
    mv -f "$TMP_OUT" "$OUT_FILE"
    echo "[rollup-cycle-metrics] wrote $OUT_FILE"
fi
