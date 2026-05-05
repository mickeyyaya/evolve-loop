#!/usr/bin/env bash
#
# failure-classifications.sh — Shared taxonomy + retention metadata (v8.22.0).
#
# WHY THIS EXISTS
#
# Pre-v8.22.0, failure entries in state.json:failedApproaches used a free-form
# "classification" string (e.g., "infrastructure", "audit-fail"). Different
# failure modes share one bucket, and the orchestrator's adaptation table
# treats them all the same. There's also no retention policy — entries from
# weeks ago permanently poison the lookback.
#
# v8.22.0 introduces:
#   1. A structured taxonomy of 7 classifications, each with metadata:
#      - severity tier (low | high | terminal)
#      - age-out window (seconds before the orchestrator filters the entry out)
#      - retry policy (yes | needs-operator | no)
#   2. Both failure-recording paths (record-failure-to-state.sh and
#      evolve-loop-dispatch.sh:record_failed_approach) source this file to
#      compute each entry's expiresAt timestamp.
#   3. failure-adapter.sh (Sprint 3) reads this file to drive its decision
#      kernel.
#
# This file is meant to be SOURCED, not executed:
#   . "$EVOLVE_PLUGIN_ROOT/scripts/failure-classifications.sh"
#
# Public API:
#   failure_age_out_seconds <classification>
#       → echos the age-out window in seconds
#   failure_severity_of <classification>
#       → echos low | high | terminal
#   failure_retry_policy <classification>
#       → echos yes | needs-operator | no
#   failure_normalize_legacy <legacy_string>
#       → echos the v8.22 classification corresponding to a pre-v8.22 string
#         (e.g., "infrastructure" → "infrastructure-transient")
#   failure_classifications_known
#       → echos newline-delimited list of recognized classifications
#
# All functions exit 0 on unknown input but emit "unknown-classification" on
# stdout (so callers can detect drift without scripts blowing up).

# Idempotency guard: this file may be sourced multiple times across nested
# script invocations.
if [ "${EVOLVE_FAILURE_CLASSIFICATIONS_LOADED:-0}" = "1" ]; then
    return 0 2>/dev/null || exit 0
fi
export EVOLVE_FAILURE_CLASSIFICATIONS_LOADED=1

# --- Age-out windows (seconds) ---------------------------------------------
# Tuning rationale:
#   - infrastructure-transient: 1 day. Sandbox EPERM / network blips resolve
#     in minutes. A day-old streak is clearly stale.
#   - infrastructure-systemic: 7 days. Tooling-missing / host-broken issues
#     persist longer; operator may take days to fix.
#   - intent-malformed: 1 day. Typically a transient prompt/model misfire.
#   - intent-rejected: never. IBTC out-of-scope is terminal — the goal must
#     change before the loop can proceed.
#   - code-build-fail / code-audit-fail: 30 days. Real code-quality evidence;
#     longer retention so similar tasks don't waste cycles re-attempting.
#   - human-abort: 1 hour. SIGTERM/SIGINT shouldn't block the next attempt.
#   - integrity-breach (legacy): 7 days. Treated as systemic infra by default.

failure_age_out_seconds() {
    case "${1:-}" in
        infrastructure-transient)  echo 86400 ;;     # 1 day
        infrastructure-systemic)   echo 604800 ;;    # 7 days
        intent-malformed)          echo 86400 ;;     # 1 day
        intent-rejected)           echo 999999999 ;; # effectively never
        code-build-fail)           echo 2592000 ;;   # 30 days
        code-audit-fail)           echo 2592000 ;;   # 30 days
        human-abort)               echo 3600 ;;      # 1 hour
        integrity-breach)          echo 604800 ;;    # 7 days (legacy/escalation)
        *)                         echo 86400 ;;     # default 1d for unknowns
    esac
}

failure_severity_of() {
    case "${1:-}" in
        infrastructure-transient|intent-malformed|human-abort) echo low ;;
        infrastructure-systemic|code-build-fail|code-audit-fail|integrity-breach) echo high ;;
        intent-rejected) echo terminal ;;
        *) echo unknown ;;
    esac
}

failure_retry_policy() {
    case "${1:-}" in
        infrastructure-transient|intent-malformed|human-abort) echo yes ;;
        infrastructure-systemic|integrity-breach) echo needs-operator ;;
        intent-rejected) echo no ;;
        # Code failures: retry only if task description differs (orchestrator
        # decides). The bare classification doesn't carry that context, so we
        # report the conservative default.
        code-build-fail|code-audit-fail) echo needs-operator ;;
        *) echo unknown ;;
    esac
}

# Migration: legacy classifications used by pre-v8.22 entries. The dispatcher's
# classify_cycle_failure used: infrastructure | audit-fail | build-fail | integrity-breach.
# record-failure-to-state.sh used verdicts: FAIL | WARN | SHIP_GATE_DENIED | WARN-NO-AUDIT | BLOCKED-*.
# This function maps both forms to the v8.22 taxonomy.
failure_normalize_legacy() {
    case "${1:-}" in
        infrastructure-transient|infrastructure-systemic|intent-malformed|intent-rejected| \
        code-build-fail|code-audit-fail|human-abort|integrity-breach)
            echo "$1" ;;
        # Legacy dispatcher classifications:
        infrastructure)        echo infrastructure-transient ;;
        audit-fail)            echo code-audit-fail ;;
        build-fail)            echo code-build-fail ;;
        # Legacy orchestrator verdicts:
        FAIL|WARN|SHIP_GATE_DENIED) echo code-audit-fail ;;
        WARN-NO-AUDIT)         echo infrastructure-systemic ;;
        BLOCKED-RECURRING-AUDIT-FAIL) echo code-audit-fail ;;
        BLOCKED-RECURRING-BUILD-FAIL) echo code-build-fail ;;
        BLOCKED-SYSTEMIC)      echo infrastructure-systemic ;;
        SCOPE-REJECTED)        echo intent-rejected ;;
        # Unknown / null:
        ""|null) echo unknown-classification ;;
        *)       echo unknown-classification ;;
    esac
}

failure_classifications_known() {
    cat <<EOF
infrastructure-transient
infrastructure-systemic
intent-malformed
intent-rejected
code-build-fail
code-audit-fail
human-abort
integrity-breach
EOF
}

# Compute an ISO-8601 expiresAt timestamp given a classification and a
# starting timestamp (defaults to now).
#
# v8.23.1 BUG FIX: pre-v8.23.1, `echo "$now_iso" | jq -r '. | fromdateiso8601'`
# silently failed because ISO timestamps without JSON quotes are NOT valid
# JSON (`2026-05-05T03:30:13Z` ← starts with a digit, jq parses as a number,
# blows up at the dash). When jq failed, `$now_s` was empty, then `(( "" + age_s ))`
# yielded `age_s`, producing a "1970-01-02T..." expiresAt — every future
# expiresAt was actually 1 day after the epoch. The fix: use `--arg` to inject
# the string with proper JSON-escaping, and refuse to silently fall back to
# epoch math if conversion fails.
#
# Usage:
#   exp=$(failure_compute_expires_at infrastructure-transient)
#   exp=$(failure_compute_expires_at code-audit-fail "2026-05-05T01:00:00Z")
failure_compute_expires_at() {
    local classification="${1:-unknown}"
    local now_iso="${2:-}"
    local age_s
    age_s=$(failure_age_out_seconds "$classification")

    # Compute target epoch.
    local now_s=""
    if [ -n "$now_iso" ]; then
        # v8.23.1: --arg injects the value as a JSON string (quoted automatically).
        # `-n` means no input — we don't pipe anything in, jq generates from --arg.
        now_s=$(jq -rn --arg s "$now_iso" '$s | fromdateiso8601' 2>/dev/null || true)
    fi
    # Fallback: use current time if conversion failed or arg was empty.
    if [ -z "$now_s" ] || ! [ "$now_s" -eq "$now_s" ] 2>/dev/null; then
        now_s=$(date -u +%s)
    fi
    local target_s=$(( now_s + age_s ))

    # Convert epoch back to ISO-8601 via jq's `todate` filter.
    # Numeric input doesn't need --arg; raw stdin works.
    echo "$target_s" | jq -r '. | todate'
}
