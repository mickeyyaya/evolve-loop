#!/usr/bin/env bash
#
# append-phase-perf.sh — Append a "## Performance & Cost" section to a
# phase report, comparing current cycle's metrics to prior cycles.
#
# Reads <agent>-timing.json + <agent>-usage.json sidecars, plus the last
# N cycles' same-phase sidecars for delta columns. Idempotent: re-running
# replaces an existing section instead of duplicating.
#
# Usage:
#   bash scripts/observability/append-phase-perf.sh <cycle> <phase>
#   bash scripts/observability/append-phase-perf.sh <cycle> <phase> --baseline=5
#   bash scripts/observability/append-phase-perf.sh <cycle> <phase> --stdout    # print, don't modify
#   bash scripts/observability/append-phase-perf.sh <cycle> <phase> --dry-run   # show what would change
#
# Exit codes:
#   0 — section appended (or printed)
#   1 — required sidecars missing
#  10 — bad arguments

set -uo pipefail

CYCLE=""
PHASE=""
BASELINE=5
STDOUT=0
DRY_RUN=0

while [ $# -gt 0 ]; do
    case "$1" in
        --baseline=*) BASELINE="${1#*=}" ;;
        --stdout) STDOUT=1 ;;
        --dry-run) DRY_RUN=1 ;;
        --help|-h) sed -n '2,22p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
        --*) echo "[append-phase-perf] unknown flag: $1" >&2; exit 10 ;;
        *)
            if [ -z "$CYCLE" ]; then CYCLE="$1"
            elif [ -z "$PHASE" ]; then PHASE="$1"
            else echo "[append-phase-perf] extra positional arg: $1" >&2; exit 10
            fi ;;
    esac
    shift
done

[ -n "$CYCLE" ] && [ -n "$PHASE" ] || { echo "[append-phase-perf] usage: append-phase-perf.sh <cycle> <phase> [--baseline=N] [--stdout|--dry-run]" >&2; exit 10; }
[[ "$CYCLE" =~ ^[0-9]+$ ]] || { echo "[append-phase-perf] cycle must be integer" >&2; exit 10; }
[[ "$BASELINE" =~ ^[0-9]+$ ]] || { echo "[append-phase-perf] --baseline must be integer" >&2; exit 10; }

command -v jq >/dev/null 2>&1 || { echo "[append-phase-perf] jq required" >&2; exit 1; }

PROJECT_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
RUNS_DIR="${RUNS_DIR_OVERRIDE:-$PROJECT_ROOT/.evolve/runs}"
WORKSPACE="$RUNS_DIR/cycle-$CYCLE"
[ -d "$WORKSPACE" ] || { echo "[append-phase-perf] no workspace at $WORKSPACE" >&2; exit 1; }

TIMING="$WORKSPACE/${PHASE}-timing.json"
USAGE="$WORKSPACE/${PHASE}-usage.json"
REPORT="$WORKSPACE/${PHASE}-report.md"

[ -f "$TIMING" ] || { echo "[append-phase-perf] no $TIMING — phase did not complete?" >&2; exit 1; }

# Read current metrics (with safe defaults if usage missing).
CUR_LATENCY_MS=$(jq -r '.total_ms // 0' "$TIMING")
CUR_COST="0"
CUR_TURNS=0
CUR_CACHE_READ=0
CUR_CACHE_CREATION=0
if [ -f "$USAGE" ]; then
    CUR_COST=$(jq -r '.total_cost_usd // 0' "$USAGE")
    CUR_TURNS=$(jq -r '.num_turns // 0' "$USAGE")
    CUR_CACHE_READ=$(jq -r '.usage.cache_read_input_tokens // 0' "$USAGE")
    CUR_CACHE_CREATION=$(jq -r '.usage.cache_creation_input_tokens // 0' "$USAGE")
fi

# Format ms → human readable.
fmt_ms_human() {
    local ms=$1
    if [ "$ms" -ge 60000 ]; then
        local sec=$((ms / 1000))
        local m=$((sec / 60))
        local s=$((sec % 60))
        printf "%dm %02ds" "$m" "$s"
    elif [ "$ms" -ge 1000 ]; then
        awk -v m="$ms" 'BEGIN { printf "%.1fs", m/1000 }'
    else
        printf "%dms" "$ms"
    fi
}

# Baseline: find last N cycles with the same phase's sidecars.
BASELINE_TIMING_TOTAL=0
BASELINE_COST_TOTAL="0"
BASELINE_TURNS_TOTAL=0
BASELINE_CACHE_READ_TOTAL=0
BASELINE_CACHE_CREATION_TOTAL=0
BASELINE_FOUND=0
PREV_CYCLE_NUM=""
PREV_LATENCY_MS=""
PREV_COST=""
PREV_TURNS=""

prev=$((CYCLE - 1))
while [ "$prev" -gt 0 ] && [ "$BASELINE_FOUND" -lt "$BASELINE" ]; do
    pt="$RUNS_DIR/cycle-$prev/${PHASE}-timing.json"
    pu="$RUNS_DIR/cycle-$prev/${PHASE}-usage.json"
    if [ -f "$pt" ]; then
        plat=$(jq -r '.total_ms // 0' "$pt" 2>/dev/null || echo 0)
        pcost="0"
        pturns=0
        pcr=0
        pcc=0
        if [ -f "$pu" ]; then
            pcost=$(jq -r '.total_cost_usd // 0' "$pu" 2>/dev/null || echo 0)
            pturns=$(jq -r '.num_turns // 0' "$pu" 2>/dev/null || echo 0)
            pcr=$(jq -r '.usage.cache_read_input_tokens // 0' "$pu" 2>/dev/null || echo 0)
            pcc=$(jq -r '.usage.cache_creation_input_tokens // 0' "$pu" 2>/dev/null || echo 0)
        fi
        BASELINE_TIMING_TOTAL=$((BASELINE_TIMING_TOTAL + plat))
        BASELINE_COST_TOTAL=$(awk -v a="$BASELINE_COST_TOTAL" -v b="$pcost" 'BEGIN { printf "%.6f", a + b }')
        BASELINE_TURNS_TOTAL=$((BASELINE_TURNS_TOTAL + pturns))
        BASELINE_CACHE_READ_TOTAL=$((BASELINE_CACHE_READ_TOTAL + pcr))
        BASELINE_CACHE_CREATION_TOTAL=$((BASELINE_CACHE_CREATION_TOTAL + pcc))
        if [ "$BASELINE_FOUND" -eq 0 ]; then
            PREV_CYCLE_NUM="$prev"
            PREV_LATENCY_MS="$plat"
            PREV_COST="$pcost"
            PREV_TURNS="$pturns"
        fi
        BASELINE_FOUND=$((BASELINE_FOUND + 1))
    fi
    prev=$((prev - 1))
done

# Delta helpers.
pct_delta() {
    # $1 = current, $2 = prior; returns "+NN%" / "-NN%" / "—"
    local cur="$1" prior="$2"
    if [ -z "$prior" ] || awk -v p="$prior" 'BEGIN { exit (p == 0) ? 0 : 1 }'; then
        echo "—"
        return
    fi
    awk -v c="$cur" -v p="$prior" 'BEGIN {
        d = (c - p) * 100 / p
        sign = (d >= 0) ? "+" : ""
        printf "%s%d%%", sign, d
    }'
}

if [ "$BASELINE_FOUND" -gt 0 ]; then
    BASELINE_AVG_LATENCY=$((BASELINE_TIMING_TOTAL / BASELINE_FOUND))
    BASELINE_AVG_COST=$(awk -v t="$BASELINE_COST_TOTAL" -v n="$BASELINE_FOUND" 'BEGIN { printf "%.6f", t / n }')
    BASELINE_AVG_TURNS=$((BASELINE_TURNS_TOTAL / BASELINE_FOUND))
else
    BASELINE_AVG_LATENCY=""
    BASELINE_AVG_COST=""
    BASELINE_AVG_TURNS=""
fi

# Cache hit rate.
cache_hit_rate_pct() {
    local r="$1" c="$2"
    local d=$((r + c))
    if [ "$d" -eq 0 ]; then echo "—"; return; fi
    awk -v r="$r" -v d="$d" 'BEGIN { printf "%d%%", r * 100 / d }'
}

CUR_CACHE_HIT=$(cache_hit_rate_pct "$CUR_CACHE_READ" "$CUR_CACHE_CREATION")
BASELINE_CACHE_HIT=$(cache_hit_rate_pct "$BASELINE_CACHE_READ_TOTAL" "$BASELINE_CACHE_CREATION_TOTAL")

# Build the markdown section.
SECTION_BEGIN="<!-- BEGIN: phase-tracker performance section -->"
SECTION_END="<!-- END: phase-tracker performance section -->"

# Format current values.
CUR_LATENCY_H=$(fmt_ms_human "$CUR_LATENCY_MS")
CUR_COST_FMT=$(awk -v c="$CUR_COST" 'BEGIN { printf "$%.4f", c }')

# Format prior + baseline (may be empty).
if [ -n "$PREV_LATENCY_MS" ]; then
    PREV_LATENCY_H=$(fmt_ms_human "$PREV_LATENCY_MS")
    PREV_COST_FMT=$(awk -v c="$PREV_COST" 'BEGIN { printf "$%.4f", c }')
    DELTA_LATENCY_PREV=$(pct_delta "$CUR_LATENCY_MS" "$PREV_LATENCY_MS")
    DELTA_COST_PREV=$(pct_delta "$CUR_COST" "$PREV_COST")
    DELTA_TURNS_PREV=$(pct_delta "$CUR_TURNS" "$PREV_TURNS")
    PREV_COL_LATENCY="$DELTA_LATENCY_PREV (was $PREV_LATENCY_H)"
    PREV_COL_COST="$DELTA_COST_PREV (was $PREV_COST_FMT)"
    PREV_COL_TURNS="$DELTA_TURNS_PREV (was $PREV_TURNS)"
    PREV_HEADER="vs cycle-$PREV_CYCLE_NUM"
else
    PREV_COL_LATENCY="—"
    PREV_COL_COST="—"
    PREV_COL_TURNS="—"
    PREV_HEADER="vs prior cycle"
fi

if [ -n "$BASELINE_AVG_LATENCY" ]; then
    BASELINE_LATENCY_H=$(fmt_ms_human "$BASELINE_AVG_LATENCY")
    BASELINE_COST_FMT=$(awk -v c="$BASELINE_AVG_COST" 'BEGIN { printf "$%.4f", c }')
    DELTA_LATENCY_BASE=$(pct_delta "$CUR_LATENCY_MS" "$BASELINE_AVG_LATENCY")
    DELTA_COST_BASE=$(pct_delta "$CUR_COST" "$BASELINE_AVG_COST")
    DELTA_TURNS_BASE=$(pct_delta "$CUR_TURNS" "$BASELINE_AVG_TURNS")
    BASE_COL_LATENCY="$DELTA_LATENCY_BASE (avg $BASELINE_LATENCY_H)"
    BASE_COL_COST="$DELTA_COST_BASE (avg $BASELINE_COST_FMT)"
    BASE_COL_TURNS="$DELTA_TURNS_BASE (avg $BASELINE_AVG_TURNS)"
    BASE_HEADER="vs last-${BASELINE_FOUND} baseline"
else
    BASE_COL_LATENCY="—"
    BASE_COL_COST="—"
    BASE_COL_TURNS="—"
    BASE_HEADER="vs baseline"
fi

SECTION=$(cat << SEC
$SECTION_BEGIN
## Performance & Cost

| Metric | This cycle | $PREV_HEADER | $BASE_HEADER |
|---|---|---|---|
| Wall time | $CUR_LATENCY_H | $PREV_COL_LATENCY | $BASE_COL_LATENCY |
| Cost | $CUR_COST_FMT | $PREV_COL_COST | $BASE_COL_COST |
| Turns | $CUR_TURNS | $PREV_COL_TURNS | $BASE_COL_TURNS |
| Cache hit rate | $CUR_CACHE_HIT | — | $BASELINE_CACHE_HIT |

_Source: \`${PHASE}-timing.json\` + \`${PHASE}-usage.json\`. Tool-call breakdown appears once Phase B wires \`--output-format stream-json\` into the dispatcher._

$SECTION_END
SEC
)

if [ "$STDOUT" = "1" ] || [ "$DRY_RUN" = "1" ]; then
    if [ "$DRY_RUN" = "1" ] && [ -f "$REPORT" ]; then
        echo "[append-phase-perf] would append to $REPORT (would replace existing section if present)"
    fi
    echo "$SECTION"
    exit 0
fi

[ -f "$REPORT" ] || { echo "[append-phase-perf] no $REPORT — phase report not generated yet" >&2; exit 1; }

# Idempotent replace: strip existing section between markers, then append fresh.
TMP_OUT="$REPORT.tmp.$$"
awk -v begin="$SECTION_BEGIN" -v end="$SECTION_END" '
    $0 == begin { skip = 1; next }
    $0 == end   { skip = 0; next }
    !skip       { print }
' "$REPORT" > "$TMP_OUT"

# Ensure trailing newline before appending.
if [ -s "$TMP_OUT" ]; then
    tail_char=$(tail -c 1 "$TMP_OUT" 2>/dev/null)
    if [ "$tail_char" != "" ]; then
        printf "\n" >> "$TMP_OUT"
    fi
fi

printf "%s\n" "$SECTION" >> "$TMP_OUT"
mv -f "$TMP_OUT" "$REPORT"
echo "[append-phase-perf] appended Performance & Cost section to $REPORT"
