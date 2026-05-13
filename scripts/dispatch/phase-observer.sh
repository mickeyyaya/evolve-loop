#!/usr/bin/env bash
#
# phase-observer.sh — Per-phase observer service.
#
# Spawned by run-cycle.sh as a sibling to each subagent. Tails the subagent's
# stream-json stdout.log, maintains L2 state in memory + temp files, runs the
# 5 deterministic detection rules periodically, and emits observations to:
#   - {phase}-observer-events.ndjson (live, append-only, one envelope/line)
#   - {phase}-observer-report.json   (atomic write at shutdown)
#
# Exits gracefully on SIGUSR1 (parent signals "subagent has exited") or when
# the stdout.log stops growing for OBSERVER_EOF_GRACE_S seconds.
#
# Args (positional):
#   $1 = workspace path (.evolve/runs/cycle-N)
#   $2 = subagent PGID (informational; observer does not kill in v1)
#   $3 = cycle number
#   $4 = phase name (research, build, etc.)
#   $5 = agent name (scout, builder, etc.)
#   $6 = cycle-state.json path (optional, for cross-reference)
#
# Env vars (all optional with sensible defaults):
#   EVOLVE_OBSERVER_POLL_S=5                  poll interval (seconds)
#   EVOLVE_OBSERVER_STALL_S=240               stuck-no-output threshold
#   EVOLVE_OBSERVER_LOOP_N=6                  infinite-loop repeat count
#   EVOLVE_OBSERVER_LOOP_WINDOW_S=120         infinite-loop rolling window
#   EVOLVE_OBSERVER_ERROR_RATE=0.3            error-spike threshold (decimal)
#   EVOLVE_OBSERVER_COST_SIGMA=2              cost-anomaly z-score threshold
#   EVOLVE_OBSERVER_THROTTLE_N=3              rate-limit events/min threshold
#   EVOLVE_OBSERVER_EOF_GRACE_S=10            wait this long after stdout
#                                              stops growing before shutdown
#
# Exit codes:
#   0 — clean shutdown, report written
#   1 — fatal error (jq missing, workspace invalid, etc.)
#  10 — bad arguments

set -uo pipefail

# ── Args ─────────────────────────────────────────────────────────────────────
WORKSPACE="${1:-}"
SUBAGENT_PGID="${2:-}"
CYCLE="${3:-}"
PHASE="${4:-}"
AGENT="${5:-}"
CYCLE_STATE="${6:-}"

[ -n "$WORKSPACE" ] && [ -n "$CYCLE" ] && [ -n "$PHASE" ] && [ -n "$AGENT" ] \
    || { echo "[phase-observer] usage: $0 <workspace> <pgid> <cycle> <phase> <agent> [cycle-state]" >&2; exit 10; }
[ -d "$WORKSPACE" ] || { echo "[phase-observer] workspace not a directory: $WORKSPACE" >&2; exit 10; }
[[ "$CYCLE" =~ ^[0-9]+$ ]] || { echo "[phase-observer] cycle must be integer" >&2; exit 10; }

command -v jq >/dev/null 2>&1 || { echo "[phase-observer] jq required" >&2; exit 1; }

# ── Locate plugin root + source libraries ──────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
PLUGIN_ROOT="${EVOLVE_PLUGIN_ROOT:-$(cd "$SCRIPT_DIR/.." && pwd -P)}"
source "$PLUGIN_ROOT/scripts/lib/severity.sh"
source "$PLUGIN_ROOT/scripts/lib/observer-rules.sh"

# ── Config ─────────────────────────────────────────────────────────────────
POLL_S="${EVOLVE_OBSERVER_POLL_S:-5}"
STALL_S="${EVOLVE_OBSERVER_STALL_S:-240}"
LOOP_N="${EVOLVE_OBSERVER_LOOP_N:-6}"
LOOP_WINDOW_S="${EVOLVE_OBSERVER_LOOP_WINDOW_S:-120}"
ERROR_RATE="${EVOLVE_OBSERVER_ERROR_RATE:-0.3}"
COST_SIGMA="${EVOLVE_OBSERVER_COST_SIGMA:-2}"
THROTTLE_N="${EVOLVE_OBSERVER_THROTTLE_N:-3}"
EOF_GRACE_S="${EVOLVE_OBSERVER_EOF_GRACE_S:-10}"

# ── Paths ──────────────────────────────────────────────────────────────────
STDOUT_LOG="$WORKSPACE/${AGENT}-stdout.log"
EVENTS_FILE="$WORKSPACE/${AGENT}-observer-events.ndjson"
REPORT_FILE="$WORKSPACE/${AGENT}-observer-report.json"
STATE_DIR=$(mktemp -d -t observer-state.XXXXXX)
LOOP_STORE="$STATE_DIR/loop-history.txt"
RATELIMIT_STORE="$STATE_DIR/ratelimit-history.txt"
TOOLHIST_STORE="$STATE_DIR/tool-histogram.txt"

# ── State ──────────────────────────────────────────────────────────────────
STARTED_AT_EPOCH=$(date +%s)
STARTED_AT_ISO=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
TRACE_ID="cycle-${CYCLE}-${PHASE}-${STARTED_AT_EPOCH}"
LAST_EVENT_TS=$STARTED_AT_EPOCH
LAST_BYTE_OFFSET=0
EVENT_COUNT=0
TOOL_CALL_COUNT=0
ERROR_COUNT=0
TOOL_RESULT_COUNT=0
RATELIMIT_COUNT=0
CUMULATIVE_COST="0"
CACHE_READ_TOKENS=0
CACHE_CREATION_TOKENS=0
INCIDENTS_FILE="$STATE_DIR/incidents.ndjson"
SHUTDOWN_REQUESTED=0
EOF_QUIET_COUNT=0   # consecutive polls with no new bytes
POLL_COUNTER=0      # for heartbeat cadence
HEARTBEAT_EVERY="${EVOLVE_OBSERVER_HEARTBEAT_EVERY:-12}"  # polls per heartbeat

# Initialize empty stores so rules don't barf on missing files.
: > "$LOOP_STORE"
: > "$RATELIMIT_STORE"
: > "$TOOLHIST_STORE"
: > "$INCIDENTS_FILE"

cleanup() {
    rm -rf "$STATE_DIR" 2>/dev/null || true
}
trap cleanup EXIT

# SIGUSR1 = "parent says subagent has exited; please finalize"
on_sigusr1() {
    SHUTDOWN_REQUESTED=1
}
trap on_sigusr1 USR1

log() { echo "[phase-observer] $*" >&2; }

# ── Envelope emitter ──────────────────────────────────────────────────────
# Wraps any payload (a verdict from a rule, or a synthesized event) in the
# unified envelope and appends to events.ndjson. Single >> append is atomic
# for ≤PIPE_BUF lines on POSIX.
emit_observation() {
    local type=$1
    local severity=$2
    local data_json=$3
    local id="obs_$(date +%s%N 2>/dev/null || date +%s)_$$_${EVENT_COUNT}"
    local envelope
    envelope=$(jq -nc \
        --arg id "$id" \
        --arg ts "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" \
        --arg trace_id "$TRACE_ID" \
        --argjson cycle "$CYCLE" \
        --arg phase "$PHASE" \
        --arg agent "$AGENT" \
        --argjson pid "$$" \
        --arg type "$type" \
        --arg severity "$severity" \
        --argjson data "$data_json" \
        '{
            id: $id,
            schema_version: "1.0",
            ts: $ts,
            trace_id: $trace_id,
            source: {
                component: "phase-observer",
                cycle: $cycle,
                phase: $phase,
                agent: $agent,
                observer_pid: $pid
            },
            type: $type,
            severity: $severity,
            data: $data
        }')
    printf '%s\n' "$envelope" >> "$EVENTS_FILE"
    # Track incidents for the phase-end report.
    if [ "$severity" = "INCIDENT" ]; then
        printf '%s\n' "$envelope" >> "$INCIDENTS_FILE"
    fi
}

# ── Tool histogram update (counts per tool_name) ────────────────────────────
tool_histogram_inc() {
    local tool=$1 is_error=$2
    # Read existing, add 1, rewrite atomically.
    local found=0 line
    local tmp="$TOOLHIST_STORE.tmp.$$"
    : > "$tmp"
    while IFS=' ' read -r count errs name; do
        [ -z "$name" ] && continue
        if [ "$name" = "$tool" ]; then
            count=$((count + 1))
            [ "$is_error" = "true" ] && errs=$((errs + 1))
            found=1
        fi
        echo "$count $errs $name" >> "$tmp"
    done < "$TOOLHIST_STORE"
    if [ "$found" = "0" ]; then
        local e=0
        [ "$is_error" = "true" ] && e=1
        echo "1 $e $tool" >> "$tmp"
    fi
    mv -f "$tmp" "$TOOLHIST_STORE"
}

# ── Process a single NDJSON line ────────────────────────────────────────────
process_line() {
    local line="$1"
    [ -z "$line" ] && return
    # Validate JSON; skip on parse failure (don't crash on malformed).
    echo "$line" | jq -e . >/dev/null 2>&1 || return

    EVENT_COUNT=$((EVENT_COUNT + 1))
    LAST_EVENT_TS=$(date +%s)

    local type
    type=$(echo "$line" | jq -r '.type // ""')

    case "$type" in
        assistant)
            local btype name input_sha
            btype=$(echo "$line" | jq -r '.message.content[0].type // ""')
            if [ "$btype" = "tool_use" ]; then
                name=$(echo "$line" | jq -r '.message.content[0].name // "?"')
                TOOL_CALL_COUNT=$((TOOL_CALL_COUNT + 1))
                input_sha=$(echo "$line" | jq -c '.message.content[0].input // {}' | shasum -a 256 | awk '{print $1}')
                echo "$LAST_EVENT_TS $input_sha $name" >> "$LOOP_STORE"
            fi
            ;;
        user)
            local rtype is_err
            rtype=$(echo "$line" | jq -r '.message.content[0].type // ""')
            if [ "$rtype" = "tool_result" ]; then
                is_err=$(echo "$line" | jq -r '.message.content[0].is_error // false')
                TOOL_RESULT_COUNT=$((TOOL_RESULT_COUNT + 1))
                [ "$is_err" = "true" ] && ERROR_COUNT=$((ERROR_COUNT + 1))
            fi
            ;;
        result)
            local cost cr cc
            cost=$(echo "$line" | jq -r '.total_cost_usd // 0')
            cr=$(echo "$line"   | jq -r '.usage.cache_read_input_tokens // 0')
            cc=$(echo "$line"   | jq -r '.usage.cache_creation_input_tokens // 0')
            CUMULATIVE_COST=$(awk -v a="$CUMULATIVE_COST" -v b="$cost" 'BEGIN { printf "%.6f", a + b }')
            CACHE_READ_TOKENS=$((CACHE_READ_TOKENS + cr))
            CACHE_CREATION_TOKENS=$((CACHE_CREATION_TOKENS + cc))
            ;;
        rate_limit_event)
            RATELIMIT_COUNT=$((RATELIMIT_COUNT + 1))
            echo "$LAST_EVENT_TS" >> "$RATELIMIT_STORE"
            ;;
    esac
}

# ── Fire all rules; emit observations on hits ──────────────────────────────
run_rules() {
    local now verdict fired sev mt
    now=$(date +%s)

    # Rule 1: stuck. Always check.
    verdict=$(rule_stuck_no_output "$now" "$LAST_EVENT_TS" "$STALL_S")
    fired=$(echo "$verdict" | jq -r '.fired')
    if [ "$fired" = "true" ]; then
        sev=$(echo "$verdict" | jq -r '.severity')
        emit_observation "observation.incident" "$sev" \
            "$(echo "$verdict" | jq -c '{metric_type, evidence, suggested_action}')"
    fi

    # Rule 2: infinite loop.
    verdict=$(rule_infinite_loop "$now" "$LOOP_STORE" "$LOOP_WINDOW_S" "$LOOP_N")
    fired=$(echo "$verdict" | jq -r '.fired')
    if [ "$fired" = "true" ]; then
        sev=$(echo "$verdict" | jq -r '.severity')
        emit_observation "observation.incident" "$sev" \
            "$(echo "$verdict" | jq -c '{metric_type, evidence, suggested_action}')"
    fi

    # Rule 3: error spike.
    verdict=$(rule_error_spike "$ERROR_COUNT" "$TOOL_RESULT_COUNT" "$ERROR_RATE")
    fired=$(echo "$verdict" | jq -r '.fired')
    if [ "$fired" = "true" ]; then
        sev=$(echo "$verdict" | jq -r '.severity')
        emit_observation "observation.warn" "$sev" \
            "$(echo "$verdict" | jq -c '{metric_type, evidence, suggested_action}')"
    fi

    # Rule 5: throttled. Count rate_limit events in last 60s.
    local cutoff=$((now - 60))
    local recent_rl
    recent_rl=$(awk -v c="$cutoff" '$1 >= c { n++ } END { print (n+0) }' "$RATELIMIT_STORE")
    verdict=$(rule_throttled "$recent_rl" 60 "$THROTTLE_N")
    fired=$(echo "$verdict" | jq -r '.fired')
    if [ "$fired" = "true" ]; then
        sev=$(echo "$verdict" | jq -r '.severity')
        emit_observation "observation.warn" "$sev" \
            "$(echo "$verdict" | jq -c '{metric_type, evidence, suggested_action}')"
    fi

    # Rule 4 (cost_anomaly) needs baseline from prior cycles; not wired in v1.
    # The hook is here; baseline computation deferred until rollup-cycle-metrics
    # exposes per-phase baselines.
}

# ── Periodic heartbeat (INFO) ──────────────────────────────────────────────
emit_heartbeat() {
    local now elapsed rate
    now=$(date +%s)
    elapsed=$((now - STARTED_AT_EPOCH))
    if [ "$elapsed" -gt 0 ]; then
        rate=$(awk -v e="$EVENT_COUNT" -v t="$elapsed" 'BEGIN { printf "%.2f", e/t }')
    else
        rate="0"
    fi
    local data
    data=$(jq -nc \
        --argjson elapsed "$elapsed" \
        --argjson events "$EVENT_COUNT" \
        --argjson tool_calls "$TOOL_CALL_COUNT" \
        --argjson errors "$ERROR_COUNT" \
        --argjson cost "$CUMULATIVE_COST" \
        --argjson rate "$rate" \
        '{
            metric_type: "heartbeat",
            elapsed_s: $elapsed,
            event_count: $events,
            tool_call_count: $tool_calls,
            error_count: $errors,
            cumulative_cost_usd: $cost,
            events_per_second: $rate
        }')
    emit_observation "observation.heartbeat" "INFO" "$data"
}

# ── Phase-end report writer ────────────────────────────────────────────────
write_phase_end_report() {
    local ended_at_epoch ended_at_iso phase_duration_ms verdict exit_reason
    ended_at_epoch=$(date +%s)
    ended_at_iso=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    phase_duration_ms=$(( (ended_at_epoch - STARTED_AT_EPOCH) * 1000 ))

    # Verdict: NORMAL / DEGRADED / INCIDENT
    local incident_count warn_count
    incident_count=$(wc -l < "$INCIDENTS_FILE" 2>/dev/null | tr -d ' ')
    [ -z "$incident_count" ] && incident_count=0
    warn_count=$(grep -c '"severity":"WARN"' "$EVENTS_FILE" 2>/dev/null)
    [ -z "$warn_count" ] && warn_count=0
    if [ "$incident_count" -gt 0 ]; then
        verdict="INCIDENT"
    elif [ "$warn_count" -gt 0 ]; then
        verdict="DEGRADED"
    else
        verdict="NORMAL"
    fi

    exit_reason="subagent_exited_normally"
    [ "$SHUTDOWN_REQUESTED" = "0" ] && exit_reason="observer_eof_grace"

    # Cache hit rate.
    local cache_total cache_hit_rate
    cache_total=$((CACHE_READ_TOKENS + CACHE_CREATION_TOKENS))
    if [ "$cache_total" -gt 0 ]; then
        cache_hit_rate=$(awk -v r="$CACHE_READ_TOKENS" -v t="$cache_total" 'BEGIN { printf "%.4f", r/t }')
    else
        cache_hit_rate="0"
    fi

    # Tool histogram as JSON object.
    local hist_json
    hist_json=$(awk '
        { printf "%s\"%s\":{\"count\":%d,\"errors\":%d}", (NR==1?"":","), $3, $1, $2 }
        END { print "" }
    ' "$TOOLHIST_STORE" | sed 's/^/{/;s/$/}/')
    [ "$hist_json" = "{}" ] && hist_json="{}"

    # Incidents array.
    local incidents_json
    if [ -s "$INCIDENTS_FILE" ]; then
        incidents_json=$(jq -s '.' "$INCIDENTS_FILE" 2>/dev/null || echo "[]")
    else
        incidents_json="[]"
    fi

    # Assemble final report.
    local report
    report=$(jq -n \
        --argjson cycle "$CYCLE" \
        --arg phase "$PHASE" \
        --arg agent "$AGENT" \
        --arg started_at "$STARTED_AT_ISO" \
        --arg ended_at "$ended_at_iso" \
        --argjson duration_ms "$phase_duration_ms" \
        --arg exit_reason "$exit_reason" \
        --arg verdict "$verdict" \
        --argjson event_count "$EVENT_COUNT" \
        --argjson tool_call_count "$TOOL_CALL_COUNT" \
        --argjson error_count "$ERROR_COUNT" \
        --argjson rate_limit_count "$RATELIMIT_COUNT" \
        --argjson cost "$CUMULATIVE_COST" \
        --argjson cache_hit_rate "$cache_hit_rate" \
        --argjson hist "$hist_json" \
        --argjson incidents "$incidents_json" \
        '{
            schema_version: "1.0",
            cycle: $cycle,
            phase: $phase,
            agent: $agent,
            observer: {
                started_at: $started_at,
                ended_at: $ended_at,
                phase_duration_ms: $duration_ms,
                exit_reason: $exit_reason
            },
            summary: {
                verdict: $verdict,
                event_count: $event_count,
                tool_call_count: $tool_call_count,
                error_count: $error_count,
                rate_limit_events: $rate_limit_count,
                cumulative_cost_usd: $cost,
                cache_hit_rate: $cache_hit_rate
            },
            incidents: $incidents,
            tool_call_histogram: $hist
        }')

    # Atomic write.
    local tmp="$REPORT_FILE.tmp.$$"
    echo "$report" > "$tmp"
    mv -f "$tmp" "$REPORT_FILE"
    log "wrote $REPORT_FILE (verdict=$verdict events=$EVENT_COUNT incidents=$incident_count)"
}

# ── Main loop ──────────────────────────────────────────────────────────────
log "started: cycle=$CYCLE phase=$PHASE agent=$AGENT workspace=$WORKSPACE poll=${POLL_S}s stall=${STALL_S}s"
log "watching $STDOUT_LOG (poll-based, byte-offset tracking)"

emit_observation "observation.heartbeat" "INFO" \
    "$(jq -nc --arg p "$PHASE" --arg a "$AGENT" '{metric_type:"start", phase:$p, agent:$a}')"

while [ "$SHUTDOWN_REQUESTED" = "0" ]; do
    if [ -f "$STDOUT_LOG" ]; then
        # Get current size in bytes.
        cur_size=""
        if cur_size=$(stat -f %z "$STDOUT_LOG" 2>/dev/null); then :
        else cur_size=$(stat -c %s "$STDOUT_LOG" 2>/dev/null || echo 0)
        fi

        if [ "$cur_size" -gt "$LAST_BYTE_OFFSET" ]; then
            # Read new bytes line-by-line. Use process substitution so state
            # updates inside process_line persist (vs a pipe-subshell).
            while IFS= read -r line; do
                process_line "$line"
            done < <(tail -c +$((LAST_BYTE_OFFSET + 1)) "$STDOUT_LOG")
            LAST_BYTE_OFFSET=$cur_size
            EOF_QUIET_COUNT=0
        else
            EOF_QUIET_COUNT=$((EOF_QUIET_COUNT + 1))
        fi
    fi

    # Fire rules every poll.
    run_rules

    # Emit heartbeat every HEARTBEAT_EVERY_N_POLLS (default 12 polls).
    POLL_COUNTER=$((POLL_COUNTER + 1))
    if [ "$POLL_COUNTER" -ge "$HEARTBEAT_EVERY" ]; then
        emit_heartbeat
        POLL_COUNTER=0
    fi

    # Auto-shutdown if stdout.log stops growing for EOF_GRACE_S seconds.
    if [ $((EOF_QUIET_COUNT * POLL_S)) -ge "$EOF_GRACE_S" ] && [ "$EVENT_COUNT" -gt 0 ]; then
        # Only auto-shutdown if we've seen at least one event (i.e., subagent
        # actually started). Otherwise wait for SIGUSR1.
        log "EOF-grace exceeded (${EOF_QUIET_COUNT} consecutive quiet polls); auto-shutdown"
        SHUTDOWN_REQUESTED=1
        break
    fi

    sleep "$POLL_S"
done

# Drain any remaining bytes before writing the report.
if [ -f "$STDOUT_LOG" ]; then
    final_size=""
    if final_size=$(stat -f %z "$STDOUT_LOG" 2>/dev/null); then :
    else final_size=$(stat -c %s "$STDOUT_LOG" 2>/dev/null || echo 0)
    fi
    if [ "$final_size" -gt "$LAST_BYTE_OFFSET" ]; then
        while IFS= read -r line; do
            process_line "$line"
        done < <(tail -c +$((LAST_BYTE_OFFSET + 1)) "$STDOUT_LOG")
    fi
fi
# One final rules pass on the drained state.
run_rules

# Emit final phase_end observation so subscribers know the stream is closing.
emit_observation "observation.phase_end" "INFO" \
    "$(jq -nc --argjson ec "$EVENT_COUNT" --argjson tc "$TOOL_CALL_COUNT" --argjson err "$ERROR_COUNT" \
        '{metric_type:"phase_end", event_count:$ec, tool_call_count:$tc, error_count:$err}')"
write_phase_end_report
log "exit 0 cycle=$CYCLE phase=$PHASE events=$EVENT_COUNT"
exit 0
