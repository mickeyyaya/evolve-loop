#!/usr/bin/env bash
#
# show-cycle-cost.sh — Per-cycle token + cost telemetry (v8.13.6).
#
# Reads each subagent's <workspace>/<agent>-stdout.log (JSON output from
# claude -p) and prints a per-phase cost breakdown for the cycle. No new
# instrumentation needed — this is a query interface over data subagent-run.sh
# has been capturing since v8.12.x.
#
# Usage:
#   bash scripts/show-cycle-cost.sh <cycle>          # human-readable table
#   bash scripts/show-cycle-cost.sh <cycle> --json   # machine-readable
#
# Example:
#   $ bash scripts/show-cycle-cost.sh 8210
#   ╭───────────────────────────────────────────────────────────────╮
#   │ Cycle 8210 cost breakdown                                     │
#   ├─────────────┬──────────┬──────────────┬──────────┬───────────┤
#   │ Phase       │   Cost $ │ Cache reads  │ Cache wr │ Out tokens│
#   ├─────────────┼──────────┼──────────────┼──────────┼───────────┤
#   │ scout       │   0.5128 │      101,097 │   39,751 │     1,533 │
#   │ auditor     │   0.6709 │      495,447 │   57,629 │     2,431 │
#   ├─────────────┼──────────┼──────────────┼──────────┼───────────┤
#   │ TOTAL       │   1.1837 │      596,544 │   97,380 │     3,964 │
#   ╰─────────────┴──────────┴──────────────┴──────────┴───────────╯
#
# Exit codes:
#   0 — at least one phase log found and parsed
#   1 — no logs found (cycle workspace missing or empty)
#  10 — bad arguments

set -uo pipefail

# v8.33.0: honor EVOLVE_PROJECT_ROOT (writable side of dual-root) so the script
# works in plugin-install scenarios where the script's directory is the plugin
# cache (read-only) and cycle data lives under the user's project. Also accept
# RUNS_DIR_OVERRIDE for tests that want a fully-isolated workspace.
PROJECT_ROOT="${EVOLVE_PROJECT_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"

CYCLE=""
JSON=0

while [ $# -gt 0 ]; do
    case "$1" in
        --json) JSON=1 ;;
        --help|-h) sed -n '2,30p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
        --*) echo "[show-cycle-cost] unknown flag: $1" >&2; exit 10 ;;
        *)
            if [ -z "$CYCLE" ]; then CYCLE="$1"
            else echo "[show-cycle-cost] extra positional arg: $1" >&2; exit 10
            fi ;;
    esac
    shift
done

[ -n "$CYCLE" ] || { echo "[show-cycle-cost] usage: show-cycle-cost.sh <cycle> [--json]" >&2; exit 10; }
[[ "$CYCLE" =~ ^[0-9]+$ ]] || { echo "[show-cycle-cost] cycle must be integer" >&2; exit 10; }

# Tests can override the runs directory directly. Otherwise compute from project root.
RUNS_DIR="${RUNS_DIR_OVERRIDE:-$PROJECT_ROOT/.evolve/runs}"
WORKSPACE="$RUNS_DIR/cycle-$CYCLE"
[ -d "$WORKSPACE" ] || { echo "[show-cycle-cost] no workspace at $WORKSPACE" >&2; exit 1; }

# Find all *-stdout.log files in the workspace.
mapfile_compat() {
    # bash 3.2-compatible mapfile replacement — read lines into the array.
    local arr_name="$1"; shift
    while IFS= read -r line; do
        eval "$arr_name+=(\"\$line\")"
    done
}

LOG_FILES=()
while IFS= read -r line; do LOG_FILES+=("$line"); done < <(find "$WORKSPACE" -maxdepth 1 -name '*-stdout.log' -type f 2>/dev/null | sort)

[ "${#LOG_FILES[@]}" -gt 0 ] || { echo "[show-cycle-cost] no *-stdout.log files in $WORKSPACE" >&2; exit 1; }

command -v jq >/dev/null 2>&1 || { echo "[show-cycle-cost] jq required" >&2; exit 1; }

# Collect per-phase rows.
PHASE_NAMES=()
COSTS=()
CACHE_READS=()
CACHE_WRITES=()
CACHE_1H=()
CACHE_5M=()
OUTPUT_TOKENS=()
INPUT_TOKENS=()

TOTAL_COST=0
TOTAL_CACHE_READ=0
TOTAL_CACHE_WRITE=0
TOTAL_CACHE_1H=0
TOTAL_CACHE_5M=0
TOTAL_OUTPUT=0
TOTAL_INPUT=0

for log in "${LOG_FILES[@]}"; do
    phase=$(basename "$log" | sed 's/-stdout\.log$//')
    # Each log MAY have multiple JSON lines (claude -p emits one final JSON
    # result per invocation; we want the LAST one which is the result-summary).
    last_json=$(tail -1 "$log" 2>/dev/null)
    [ -n "$last_json" ] || continue
    # Parse fields, defaulting to 0 if absent.
    cost=$(echo "$last_json" | jq -r '.total_cost_usd // 0' 2>/dev/null || echo 0)
    cache_read=$(echo "$last_json" | jq -r '.usage.cache_read_input_tokens // 0' 2>/dev/null || echo 0)
    cache_write=$(echo "$last_json" | jq -r '.usage.cache_creation_input_tokens // 0' 2>/dev/null || echo 0)
    # TTL bucket split — claude -p emits these nested under usage.cache_creation
    # when the API returns the per-bucket breakdown. Legacy logs omit the
    # nested object; default to 0 so existing telemetry continues to render.
    cache_1h=$(echo "$last_json" | jq -r '.usage.cache_creation.ephemeral_1h_input_tokens // 0' 2>/dev/null || echo 0)
    cache_5m=$(echo "$last_json" | jq -r '.usage.cache_creation.ephemeral_5m_input_tokens // 0' 2>/dev/null || echo 0)
    output_t=$(echo "$last_json" | jq -r '.usage.output_tokens // 0' 2>/dev/null || echo 0)
    input_t=$(echo "$last_json" | jq -r '.usage.input_tokens // 0' 2>/dev/null || echo 0)

    PHASE_NAMES+=("$phase")
    COSTS+=("$cost")
    CACHE_READS+=("$cache_read")
    CACHE_WRITES+=("$cache_write")
    CACHE_1H+=("$cache_1h")
    CACHE_5M+=("$cache_5m")
    OUTPUT_TOKENS+=("$output_t")
    INPUT_TOKENS+=("$input_t")

    # Bash 3.2 has no decimal arithmetic; use bc for cost, integers for tokens.
    TOTAL_COST=$(echo "$TOTAL_COST + $cost" | bc -l 2>/dev/null || echo "$TOTAL_COST")
    TOTAL_CACHE_READ=$((TOTAL_CACHE_READ + cache_read))
    TOTAL_CACHE_WRITE=$((TOTAL_CACHE_WRITE + cache_write))
    TOTAL_CACHE_1H=$((TOTAL_CACHE_1H + cache_1h))
    TOTAL_CACHE_5M=$((TOTAL_CACHE_5M + cache_5m))
    TOTAL_OUTPUT=$((TOTAL_OUTPUT + output_t))
    TOTAL_INPUT=$((TOTAL_INPUT + input_t))
done

# --- Output ----------------------------------------------------------------

if [ "$JSON" = "1" ]; then
    # Build JSON via jq directly. We pass the arrays as separate jq args.
    json_phases=$(printf '%s\n' "${PHASE_NAMES[@]}" | jq -R . | jq -s .)
    json_costs=$(printf '%s\n' "${COSTS[@]}" | jq -R 'tonumber' | jq -s .)
    json_cache_reads=$(printf '%s\n' "${CACHE_READS[@]}" | jq -R 'tonumber' | jq -s .)
    json_cache_writes=$(printf '%s\n' "${CACHE_WRITES[@]}" | jq -R 'tonumber' | jq -s .)
    json_cache_1h=$(printf '%s\n' "${CACHE_1H[@]}" | jq -R 'tonumber' | jq -s .)
    json_cache_5m=$(printf '%s\n' "${CACHE_5M[@]}" | jq -R 'tonumber' | jq -s .)
    json_outputs=$(printf '%s\n' "${OUTPUT_TOKENS[@]}" | jq -R 'tonumber' | jq -s .)
    json_inputs=$(printf '%s\n' "${INPUT_TOKENS[@]}" | jq -R 'tonumber' | jq -s .)
    jq -nc \
        --argjson cycle "$CYCLE" \
        --argjson phases "$json_phases" \
        --argjson costs "$json_costs" \
        --argjson cache_reads "$json_cache_reads" \
        --argjson cache_writes "$json_cache_writes" \
        --argjson cache_1h "$json_cache_1h" \
        --argjson cache_5m "$json_cache_5m" \
        --argjson outputs "$json_outputs" \
        --argjson inputs "$json_inputs" \
        --arg total_cost "$TOTAL_COST" \
        --argjson total_cache_read "$TOTAL_CACHE_READ" \
        --argjson total_cache_write "$TOTAL_CACHE_WRITE" \
        --argjson total_cache_1h "$TOTAL_CACHE_1H" \
        --argjson total_cache_5m "$TOTAL_CACHE_5M" \
        --argjson total_output "$TOTAL_OUTPUT" \
        --argjson total_input "$TOTAL_INPUT" \
        '{
            cycle: $cycle,
            phases: ([range(0; $phases | length)] | map({
                phase: $phases[.],
                cost_usd: $costs[.],
                cache_read_input_tokens: $cache_reads[.],
                cache_creation_input_tokens: $cache_writes[.],
                cache_creation_1h_input_tokens: $cache_1h[.],
                cache_creation_5m_input_tokens: $cache_5m[.],
                output_tokens: $outputs[.],
                input_tokens: $inputs[.]
            })),
            total: {
                cost_usd: $total_cost | tonumber,
                cache_read_input_tokens: $total_cache_read,
                cache_creation_input_tokens: $total_cache_write,
                cache_creation_1h_input_tokens: $total_cache_1h,
                cache_creation_5m_input_tokens: $total_cache_5m,
                output_tokens: $total_output,
                input_tokens: $total_input
            }
        }'
    exit 0
fi

# Human-readable table.
printf '%s\n' "Cycle $CYCLE cost breakdown ($WORKSPACE)"
printf '┌─────────────────┬──────────┬──────────────┬──────────────┬────────────┬────────────┐\n'
printf '│ %-15s │ %8s │ %12s │ %12s │ %10s │ %10s │\n' "Phase" "Cost \$" "Cache reads" "Cache writes" "Out tokens" "In tokens"
printf '├─────────────────┼──────────┼──────────────┼──────────────┼────────────┼────────────┤\n'

i=0
for phase in "${PHASE_NAMES[@]}"; do
    cost=${COSTS[$i]}
    cr=${CACHE_READS[$i]}
    cw=${CACHE_WRITES[$i]}
    ot=${OUTPUT_TOKENS[$i]}
    it=${INPUT_TOKENS[$i]}
    # Format with thousands separator for token counts.
    cr_fmt=$(printf "%'d" "$cr" 2>/dev/null || echo "$cr")
    cw_fmt=$(printf "%'d" "$cw" 2>/dev/null || echo "$cw")
    ot_fmt=$(printf "%'d" "$ot" 2>/dev/null || echo "$ot")
    it_fmt=$(printf "%'d" "$it" 2>/dev/null || echo "$it")
    printf '│ %-15s │ %8.4f │ %12s │ %12s │ %10s │ %10s │\n' "$phase" "$cost" "$cr_fmt" "$cw_fmt" "$ot_fmt" "$it_fmt"
    i=$((i + 1))
done

printf '├─────────────────┼──────────┼──────────────┼──────────────┼────────────┼────────────┤\n'
total_cr_fmt=$(printf "%'d" "$TOTAL_CACHE_READ" 2>/dev/null || echo "$TOTAL_CACHE_READ")
total_cw_fmt=$(printf "%'d" "$TOTAL_CACHE_WRITE" 2>/dev/null || echo "$TOTAL_CACHE_WRITE")
total_ot_fmt=$(printf "%'d" "$TOTAL_OUTPUT" 2>/dev/null || echo "$TOTAL_OUTPUT")
total_it_fmt=$(printf "%'d" "$TOTAL_INPUT" 2>/dev/null || echo "$TOTAL_INPUT")
printf '│ %-15s │ %8.4f │ %12s │ %12s │ %10s │ %10s │\n' "TOTAL" "$TOTAL_COST" "$total_cr_fmt" "$total_cw_fmt" "$total_ot_fmt" "$total_it_fmt"
printf '└─────────────────┴──────────┴──────────────┴──────────────┴────────────┴────────────┘\n'

# Cache TTL bucket split — surfaces whether cache writes hit the 1-hour or
# 5-minute bucket. On Max-tier subscriptions the 1h bucket dominates (cheaper
# amortized writes); a sudden shift toward 5m signals an Anthropic-side flag
# regression (see GitHub anthropics/claude-code#46829, March 2026 incident).
# Zero in both buckets means legacy logs without the nested cache_creation
# breakdown — not a failure mode, just older telemetry.
total_1h_fmt=$(printf "%'d" "$TOTAL_CACHE_1H" 2>/dev/null || echo "$TOTAL_CACHE_1H")
total_5m_fmt=$(printf "%'d" "$TOTAL_CACHE_5M" 2>/dev/null || echo "$TOTAL_CACHE_5M")
printf 'Cache TTL split: 1h=%s  5m=%s\n' "$total_1h_fmt" "$total_5m_fmt"

exit 0
