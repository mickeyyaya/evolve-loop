#!/usr/bin/env bash
#
# tracker-writer.sh — stream-json parser → NDJSON tracker + trace.md +
# per-phase metrics. Phase-B scaffolding: this script is wired into the
# dispatch pipeline only when EVOLVE_TRACKER_ENABLED=1 and claude.sh is
# updated to emit --output-format stream-json. In Phase A it is exercised
# by scripts/tests/tracker-writer-test.sh against synthetic fixtures.
#
# Reads one JSON event per stdin line. Events that fail JSON parse are
# logged to stderr and dropped (the tracker should never crash a live
# subagent run).
#
# Usage:
#   <stream-json source> | tracker-writer.sh --cycle=N --phase=P --invocation-id=ID
#
# Required:
#   --cycle=N             Cycle number
#   --phase=NAME          Phase / agent role (scout, builder, auditor, …)
#   --invocation-id=ID    Per-invocation token (used in tracker filename)
#
# Optional:
#   --runs-dir=PATH       Override .evolve/runs base
#   --no-trace            Skip trace.md (NDJSON only)
#   --no-metrics          Skip metrics/PHASE.json (NDJSON only)
#
# Exit codes:
#   0 — clean EOF
#   1 — fatal IO error
#  10 — bad arguments

set -uo pipefail

CYCLE=""
PHASE=""
INVOCATION_ID=""
RUNS_DIR_OVERRIDE_FLAG=""
WRITE_TRACE=1
WRITE_METRICS=1

while [ $# -gt 0 ]; do
    case "$1" in
        --cycle=*) CYCLE="${1#*=}" ;;
        --phase=*) PHASE="${1#*=}" ;;
        --invocation-id=*) INVOCATION_ID="${1#*=}" ;;
        --runs-dir=*) RUNS_DIR_OVERRIDE_FLAG="${1#*=}" ;;
        --no-trace) WRITE_TRACE=0 ;;
        --no-metrics) WRITE_METRICS=0 ;;
        --help|-h) sed -n '2,25p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
        --*) echo "[tracker-writer] unknown flag: $1" >&2; exit 10 ;;
        *) echo "[tracker-writer] unexpected positional arg: $1" >&2; exit 10 ;;
    esac
    shift
done

[ -n "$CYCLE" ] || { echo "[tracker-writer] --cycle required" >&2; exit 10; }
[ -n "$PHASE" ] || { echo "[tracker-writer] --phase required" >&2; exit 10; }
[ -n "$INVOCATION_ID" ] || { echo "[tracker-writer] --invocation-id required" >&2; exit 10; }
[[ "$CYCLE" =~ ^[0-9]+$ ]] || { echo "[tracker-writer] cycle must be integer" >&2; exit 10; }

command -v jq >/dev/null 2>&1 || { echo "[tracker-writer] jq required" >&2; exit 1; }

PROJECT_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
if [ -n "$RUNS_DIR_OVERRIDE_FLAG" ]; then
    RUNS_DIR="$RUNS_DIR_OVERRIDE_FLAG"
else
    RUNS_DIR="${RUNS_DIR_OVERRIDE:-$PROJECT_ROOT/.evolve/runs}"
fi

WORKSPACE="$RUNS_DIR/cycle-$CYCLE"
EPHEMERAL="$WORKSPACE/.ephemeral"
TRACKER_DIR="$EPHEMERAL/trackers"
METRICS_DIR="$EPHEMERAL/metrics"
TRACE_FILE="$EPHEMERAL/trace.md"
TRACKER_FILE="$TRACKER_DIR/${PHASE}-${INVOCATION_ID}.ndjson"
METRICS_FILE="$METRICS_DIR/${PHASE}.json"

mkdir -p "$TRACKER_DIR" "$METRICS_DIR" || { echo "[tracker-writer] failed to mkdir $EPHEMERAL" >&2; exit 1; }

# In-memory tally. Bash 3.2 — flat scalars, no associative arrays. For
# tool_use → tool_result latency, use a temp file as key→ts store.
LATENCY_STORE=$(mktemp -t tracker-lat.XXXXXX)
trap 'rm -f "$LATENCY_STORE"' EXIT

# Tool histogram via parallel arrays.
TOOL_NAMES=()
TOOL_COUNTS=()
TOOL_TOTAL_MS=()

tool_inc() {
    local name=$1 ms=$2
    local i
    for i in "${!TOOL_NAMES[@]}"; do
        if [ "${TOOL_NAMES[$i]}" = "$name" ]; then
            TOOL_COUNTS[$i]=$((TOOL_COUNTS[$i] + 1))
            TOOL_TOTAL_MS[$i]=$((TOOL_TOTAL_MS[$i] + ms))
            return
        fi
    done
    TOOL_NAMES+=("$name")
    TOOL_COUNTS+=("1")
    TOOL_TOTAL_MS+=("$ms")
}

# Stats.
TURN=0
EVENT_COUNT=0
TOTAL_COST="0"
TOTAL_DURATION_MS=0
NUM_TURNS=0
STOP_REASON=""
SESSION_ID=""
MODEL=""
PHASE_STARTED_AT=""
PHASE_ENDED_AT=""

# Detect GNU vs BSD/macOS date once (BSD/macOS has no %N for nanosec).
_DATE_HAS_N=0
if date -u +"%3N" 2>/dev/null | grep -qE '^[0-9]{1,3}$'; then
    _DATE_HAS_N=1
fi

now_iso() {
    if [ "$_DATE_HAS_N" = "1" ]; then
        date -u +"%Y-%m-%dT%H:%M:%S.%3NZ"
    else
        date -u +"%Y-%m-%dT%H:%M:%SZ"
    fi
}

epoch_ms() {
    # Milliseconds since epoch (best-effort).
    if command -v gdate >/dev/null 2>&1; then
        gdate +%s%3N
    else
        # macOS: seconds × 1000, no millis available.
        echo "$(( $(date +%s) * 1000 ))"
    fi
}

# trace.md append helper. Skips when --no-trace.
trace_append() {
    [ "$WRITE_TRACE" = "1" ] || return 0
    printf "%s\n" "$1" >> "$TRACE_FILE"
}

# Strict JSON parse via jq. Returns 0 if valid, 1 otherwise.
is_json() {
    echo "$1" | jq -e . >/dev/null 2>&1
}

# Phase start banner.
PHASE_STARTED_AT=$(now_iso)
trace_append "[$(echo "$PHASE_STARTED_AT" | cut -c12-19)] cycle-$CYCLE $(printf '%-12s' "$PHASE") START   invocation=$INVOCATION_ID"

while IFS= read -r line; do
    [ -z "$line" ] && continue
    EVENT_COUNT=$((EVENT_COUNT + 1))

    # Validate JSON; on failure, log to stderr and continue.
    if ! is_json "$line"; then
        echo "[tracker-writer] skipping non-JSON line $EVENT_COUNT: $(echo "$line" | head -c 80)" >&2
        continue
    fi

    # Stamp + augment.
    TS=$(now_iso)
    EVT_TYPE=$(echo "$line" | jq -r '.type // "unknown"')

    AUGMENTED=$(echo "$line" | jq -c \
        --arg ts "$TS" \
        --argjson cycle "$CYCLE" \
        --arg phase "$PHASE" \
        --arg invocation_id "$INVOCATION_ID" \
        '. + {ts: $ts, cycle: $cycle, phase: $phase, invocation_id: $invocation_id}')

    # Append raw event to NDJSON file (atomic for small lines).
    printf "%s\n" "$AUGMENTED" >> "$TRACKER_FILE"

    # Type-specific handling.
    case "$EVT_TYPE" in
        system)
            sub=$(echo "$line" | jq -r '.subtype // ""')
            sess=$(echo "$line" | jq -r '.session_id // ""')
            mdl=$(echo "$line" | jq -r '.model // ""')
            [ -n "$sess" ] && SESSION_ID="$sess"
            [ -n "$mdl" ] && MODEL="$mdl"
            trace_append "[$(echo "$TS" | cut -c12-19)] cycle-$CYCLE $(printf '%-12s' "$PHASE") sys     $sub session=$sess model=$mdl"
            ;;
        assistant)
            TURN=$((TURN + 1))
            # Extract text or tool_use from the first content block.
            block_type=$(echo "$line" | jq -r '.message.content[0].type // ""')
            if [ "$block_type" = "text" ]; then
                text_preview=$(echo "$line" | jq -r '.message.content[0].text // ""' | head -c 80 | tr '\n' ' ')
                trace_append "[$(echo "$TS" | cut -c12-19)] cycle-$CYCLE $(printf '%-12s' "$PHASE") t$TURN msg   \"$text_preview\""
            elif [ "$block_type" = "tool_use" ]; then
                tool_name=$(echo "$line" | jq -r '.message.content[0].name // "?"')
                tool_use_id=$(echo "$line" | jq -r '.message.content[0].id // ""')
                tool_input_preview=$(echo "$line" | jq -c '.message.content[0].input // {}' | head -c 60)
                # Store tool_use ts for latency calc on the matching tool_result.
                echo "$tool_use_id $(epoch_ms)" >> "$LATENCY_STORE"
                trace_append "[$(echo "$TS" | cut -c12-19)] cycle-$CYCLE $(printf '%-12s' "$PHASE") t$TURN tool  $tool_name $tool_input_preview"
            fi
            ;;
        user)
            # tool_result inside a user message.
            result_block_type=$(echo "$line" | jq -r '.message.content[0].type // ""')
            if [ "$result_block_type" = "tool_result" ]; then
                tool_use_id=$(echo "$line" | jq -r '.message.content[0].tool_use_id // ""')
                is_err=$(echo "$line" | jq -r '.message.content[0].is_error // false')
                # Latency lookup.
                start_ms=""
                if [ -n "$tool_use_id" ]; then
                    start_ms=$(grep "^$tool_use_id " "$LATENCY_STORE" 2>/dev/null | tail -1 | awk '{print $2}')
                fi
                latency_ms=0
                if [ -n "$start_ms" ]; then
                    end_ms=$(epoch_ms)
                    latency_ms=$((end_ms - start_ms))
                fi
                # Compute size_bytes from the content.
                size_bytes=$(echo "$line" | jq -r '.message.content[0].content // "" | tostring | length' 2>/dev/null || echo 0)
                # Find matching tool name from latency store sidecar (if recorded).
                # For accuracy, re-scan the tracker file for the most recent
                # tool_use with this id and pull its name. Best-effort.
                tool_name=$(grep -F "\"$tool_use_id\"" "$TRACKER_FILE" 2>/dev/null \
                    | jq -r 'select(.type=="assistant") | .message.content[0].name // empty' 2>/dev/null \
                    | tail -1)
                [ -z "$tool_name" ] && tool_name="?"
                tool_inc "$tool_name" "$latency_ms"
                latency_h="${latency_ms}ms"
                if [ "$latency_ms" -ge 60000 ]; then
                    latency_h=$(awk -v m="$latency_ms" 'BEGIN { s=int(m/1000); printf "%dm%02ds", int(s/60), s%60 }')
                elif [ "$latency_ms" -ge 1000 ]; then
                    latency_h=$(awk -v m="$latency_ms" 'BEGIN { printf "%.1fs", m/1000 }')
                fi
                err_flag=""
                [ "$is_err" = "true" ] && err_flag=" ERROR"
                trace_append "[$(echo "$TS" | cut -c12-19)] cycle-$CYCLE $(printf '%-12s' "$PHASE") t$TURN ←     $tool_name [$latency_h] ${size_bytes}B$err_flag"
            fi
            ;;
        result)
            # Final summary event.
            TOTAL_COST=$(echo "$line" | jq -r '.total_cost_usd // 0')
            TOTAL_DURATION_MS=$(echo "$line" | jq -r '.duration_ms // 0')
            NUM_TURNS=$(echo "$line" | jq -r '.num_turns // 0')
            STOP_REASON=$(echo "$line" | jq -r '.stop_reason // .subtype // "unknown"')
            ;;
        error)
            msg=$(echo "$line" | jq -r '.message // "(no message)"')
            trace_append "[$(echo "$TS" | cut -c12-19)] cycle-$CYCLE $(printf '%-12s' "$PHASE") ERROR $msg"
            ;;
        *)
            # Unknown type — augmented event already in NDJSON; nothing extra.
            ;;
    esac
done

PHASE_ENDED_AT=$(now_iso)
TOTAL_COST_FMT=$(awk -v c="$TOTAL_COST" 'BEGIN { printf "$%.4f", c }')
DURATION_H="${TOTAL_DURATION_MS}ms"
if [ "$TOTAL_DURATION_MS" -ge 60000 ]; then
    DURATION_H=$(awk -v m="$TOTAL_DURATION_MS" 'BEGIN { s=int(m/1000); printf "%dm%02ds", int(s/60), s%60 }')
elif [ "$TOTAL_DURATION_MS" -ge 1000 ]; then
    DURATION_H=$(awk -v m="$TOTAL_DURATION_MS" 'BEGIN { printf "%.1fs", m/1000 }')
fi

trace_append "[$(echo "$PHASE_ENDED_AT" | cut -c12-19)] cycle-$CYCLE $(printf '%-12s' "$PHASE") END     latency=$DURATION_H cost=$TOTAL_COST_FMT turns=$NUM_TURNS stop=$STOP_REASON"

# Build per-tool histogram JSON.
TOOL_HIST_TMP=$(mktemp -t tracker-toolhist.XXXXXX)
for i in "${!TOOL_NAMES[@]}"; do
    name="${TOOL_NAMES[$i]}"
    count="${TOOL_COUNTS[$i]}"
    total_ms="${TOOL_TOTAL_MS[$i]}"
    avg_ms=0
    if [ "$count" -gt 0 ]; then avg_ms=$((total_ms / count)); fi
    jq -nc \
        --arg name "$name" \
        --argjson count "$count" \
        --argjson total_ms "$total_ms" \
        --argjson avg_ms "$avg_ms" \
        '{name: $name, count: $count, total_ms: $total_ms, avg_ms: $avg_ms}' >> "$TOOL_HIST_TMP"
done
TOOL_HIST_JSON=$(jq -s 'sort_by(-.total_ms)' "$TOOL_HIST_TMP" 2>/dev/null || echo "[]")
rm -f "$TOOL_HIST_TMP"

# Write metrics snapshot atomically.
if [ "$WRITE_METRICS" = "1" ]; then
    METRICS_JSON=$(jq -nc \
        --arg phase "$PHASE" \
        --argjson cycle "$CYCLE" \
        --arg invocation_id "$INVOCATION_ID" \
        --arg started_at "$PHASE_STARTED_AT" \
        --arg ended_at "$PHASE_ENDED_AT" \
        --argjson duration_ms "$TOTAL_DURATION_MS" \
        --argjson cost_usd "$TOTAL_COST" \
        --argjson num_turns "$NUM_TURNS" \
        --argjson event_count "$EVENT_COUNT" \
        --arg session_id "$SESSION_ID" \
        --arg model "$MODEL" \
        --arg stop_reason "$STOP_REASON" \
        --argjson tool_calls "$TOOL_HIST_JSON" \
        '{schema_version: "1.0", phase: $phase, cycle: $cycle, invocation_id: $invocation_id, started_at: $started_at, ended_at: $ended_at, duration_ms: $duration_ms, cost_usd: $cost_usd, num_turns: $num_turns, event_count: $event_count, session_id: $session_id, model: $model, stop_reason: $stop_reason, tool_calls: $tool_calls}')
    TMP_OUT="$METRICS_FILE.tmp.$$"
    echo "$METRICS_JSON" | jq '.' > "$TMP_OUT"
    mv -f "$TMP_OUT" "$METRICS_FILE"
fi

echo "[tracker-writer] phase=$PHASE cycle=$CYCLE events=$EVENT_COUNT turns=$NUM_TURNS cost=$TOTAL_COST_FMT duration=$DURATION_H tracker=$TRACKER_FILE"
