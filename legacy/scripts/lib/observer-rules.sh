#!/usr/bin/env bash
#
# observer-rules.sh — Detection rule library for phase-observer.
#
# Each rule is a pure function that:
#   - Takes L2 state as args
#   - Reads a temp-file state store when needed (loop history, etc.)
#   - Emits a JSON verdict on stdout: {"fired": bool, severity?, metric_type?,
#     evidence?, suggested_action?}
#   - Returns 0 (does not signal fire via exit code; check the JSON `.fired`)
#
# All thresholds env-var-tunable. Bash 3.2 compatible — no associative arrays.
#
# Usage (from phase-observer.sh):
#   source "$EVOLVE_PLUGIN_ROOT/scripts/lib/observer-rules.sh"
#   verdict=$(rule_stuck_no_output "$now" "$last_event_ts" "$STALL_THRESHOLD_S")
#   if [ "$(echo "$verdict" | jq -r '.fired')" = "true" ]; then
#       emit_observation "$verdict"
#   fi

if [ -n "${EVOLVE_OBSERVER_RULES_SH_LOADED:-}" ]; then
    return 0 2>/dev/null || exit 0
fi
EVOLVE_OBSERVER_RULES_SH_LOADED=1

# ── Rule 1: stuck_no_output ─────────────────────────────────────────────────
# Fires when no observation events have arrived for >= threshold seconds.
# This is the watchdog's job, but emitted as an INCIDENT observation rather
# than a kill signal. Phase-watchdog still does the SIGTERM in v1.
#
# Args: $1=now_epoch $2=last_event_epoch $3=threshold_seconds
rule_stuck_no_output() {
    local now=$1 last=$2 thresh=$3
    local idle=$((now - last))
    if [ "$idle" -ge "$thresh" ]; then
        jq -nc \
            --argjson idle "$idle" \
            --argjson thresh "$thresh" \
            '{
                fired: true,
                severity: "INCIDENT",
                metric_type: "stuck",
                evidence: {
                    idle_seconds: $idle,
                    threshold_seconds: $thresh
                },
                suggested_action: {
                    action_type: "terminate_phase",
                    reason_code: "STUCK_NO_OUTPUT",
                    human_readable: ("Phase produced no events for " + ($idle | tostring) + "s (threshold: " + ($thresh | tostring) + "s). Recommended: terminate phase and inspect.")
                }
            }'
    else
        echo '{"fired":false}'
    fi
}

# ── Rule 2: infinite_loop ───────────────────────────────────────────────────
# Fires when the same (tool_name + args_sha256) appears >= N times within a
# rolling window. Detects: agent stuck re-Reading the same file, re-running
# the same Bash, etc.
#
# State store: lines in $loop_store as `<epoch> <sha256> <tool_name>`.
# Caller is responsible for appending each tool_use to the store.
#
# Args: $1=now_epoch $2=loop_store_path $3=window_seconds $4=repeat_n
rule_infinite_loop() {
    local now=$1 store=$2 window=$3 threshold=$4
    [ -f "$store" ] || { echo '{"fired":false}'; return 0; }

    local cutoff=$((now - window))
    # Count occurrences of the most-recent sha256 within the window.
    # Bash 3.2: use awk for the aggregation.
    local hit
    hit=$(awk -v cutoff="$cutoff" '
        $1 >= cutoff {
            counts[$2]++
            tools[$2] = $3
            if (counts[$2] > max) {
                max = counts[$2]
                top_sha = $2
                top_tool = tools[$2]
            }
        }
        END {
            if (max > 0) printf "%d %s %s\n", max, top_sha, top_tool
        }
    ' "$store")

    if [ -z "$hit" ]; then
        echo '{"fired":false}'
        return 0
    fi

    local count sha tool
    count=$(echo "$hit" | awk '{print $1}')
    sha=$(echo "$hit"   | awk '{print $2}')
    tool=$(echo "$hit"  | awk '{print $3}')

    if [ "$count" -ge "$threshold" ]; then
        jq -nc \
            --argjson count "$count" \
            --arg sha "$sha" \
            --arg tool "$tool" \
            --argjson window "$window" \
            --argjson thresh "$threshold" \
            '{
                fired: true,
                severity: "INCIDENT",
                metric_type: "infinite_loop",
                evidence: {
                    tool_name: $tool,
                    args_sha256: $sha,
                    repeat_count: $count,
                    window_seconds: $window,
                    threshold: $thresh
                },
                suggested_action: {
                    action_type: "terminate_phase",
                    reason_code: "INFINITE_LOOP",
                    human_readable: ("Agent called " + $tool + " with identical args " + ($count | tostring) + " times in " + ($window | tostring) + "s window (threshold: " + ($thresh | tostring) + "). Recommended: terminate phase and inspect why the agent is repeating.")
                }
            }'
    else
        echo '{"fired":false}'
    fi
}

# ── Rule 3: error_spike ─────────────────────────────────────────────────────
# Fires when tool_result.is_error rate exceeds the configured fraction.
#
# Args: $1=error_count $2=total_tool_result_count $3=error_rate_threshold
#       (a decimal between 0 and 1, e.g. 0.3)
rule_error_spike() {
    local errors=$1 total=$2 threshold=$3
    [ "$total" -lt 5 ] && { echo '{"fired":false}'; return 0; }   # need sample size

    # Compute rate via awk (bash has no float).
    local rate fired
    rate=$(awk -v e="$errors" -v t="$total" 'BEGIN { printf "%.3f", e/t }')
    fired=$(awk -v r="$rate" -v th="$threshold" 'BEGIN { print (r >= th) ? "true" : "false" }')

    if [ "$fired" = "true" ]; then
        jq -nc \
            --argjson errors "$errors" \
            --argjson total "$total" \
            --argjson rate "$rate" \
            --argjson thresh "$threshold" \
            '{
                fired: true,
                severity: "WARN",
                metric_type: "error_spike",
                evidence: {
                    error_count: $errors,
                    total_tool_results: $total,
                    error_rate: $rate,
                    threshold: $thresh
                },
                suggested_action: {
                    action_type: "continue",
                    reason_code: "ERROR_SPIKE",
                    human_readable: ("Tool error rate is " + ($rate | tostring) + " (" + ($errors | tostring) + "/" + ($total | tostring) + ") — above threshold " + ($thresh | tostring) + ". Phase continues; flag for review.")
                }
            }'
    else
        echo '{"fired":false}'
    fi
}

# ── Rule 4: cost_anomaly ────────────────────────────────────────────────────
# Fires when cumulative cost exceeds baseline p95 by N sigma.
# Baseline computed from prior cycles' same-phase costs.
#
# Args: $1=current_cost $2=baseline_mean $3=baseline_stddev $4=sigma_threshold
# (If baseline_stddev is 0 or missing, this rule does not fire.)
rule_cost_anomaly() {
    local cur=$1 mean=$2 stddev=$3 sigma=$4
    # No baseline → no fire (first cycle, etc.)
    local nonzero
    nonzero=$(awk -v s="$stddev" 'BEGIN { print (s > 0) ? "1" : "0" }')
    [ "$nonzero" = "0" ] && { echo '{"fired":false}'; return 0; }

    local z fired
    z=$(awk -v c="$cur" -v m="$mean" -v s="$stddev" 'BEGIN { printf "%.2f", (c - m) / s }')
    fired=$(awk -v z="$z" -v t="$sigma" 'BEGIN { print (z >= t) ? "true" : "false" }')

    if [ "$fired" = "true" ]; then
        jq -nc \
            --argjson cur "$cur" \
            --argjson mean "$mean" \
            --argjson stddev "$stddev" \
            --argjson z "$z" \
            --argjson sigma "$sigma" \
            '{
                fired: true,
                severity: "WARN",
                metric_type: "cost_anomaly",
                evidence: {
                    current_cost_usd: $cur,
                    baseline_mean_usd: $mean,
                    baseline_stddev_usd: $stddev,
                    z_score: $z,
                    sigma_threshold: $sigma
                },
                suggested_action: {
                    action_type: "continue",
                    reason_code: "COST_ANOMALY",
                    human_readable: ("Cumulative cost $" + ($cur | tostring) + " is " + ($z | tostring) + "σ above baseline mean $" + ($mean | tostring) + " (threshold: " + ($sigma | tostring) + "σ). Phase continues; flag for operator review.")
                }
            }'
    else
        echo '{"fired":false}'
    fi
}

# ── Rule 5: throttled ───────────────────────────────────────────────────────
# Fires when rate_limit_event count crosses threshold within a rolling window.
#
# Args: $1=rate_limit_count_in_window $2=window_seconds $3=threshold_n
rule_throttled() {
    local count=$1 window=$2 threshold=$3
    if [ "$count" -ge "$threshold" ]; then
        jq -nc \
            --argjson count "$count" \
            --argjson window "$window" \
            --argjson thresh "$threshold" \
            '{
                fired: true,
                severity: "WARN",
                metric_type: "throttled",
                evidence: {
                    rate_limit_events: $count,
                    window_seconds: $window,
                    threshold: $thresh
                },
                suggested_action: {
                    action_type: "continue",
                    reason_code: "API_THROTTLED",
                    human_readable: ("API rate-limited " + ($count | tostring) + " times in " + ($window | tostring) + "s (threshold: " + ($thresh | tostring) + "). Phase continues; suggest backoff window before next cycle.")
                }
            }'
    else
        echo '{"fired":false}'
    fi
}
