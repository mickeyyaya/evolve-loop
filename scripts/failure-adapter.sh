#!/usr/bin/env bash
#
# failure-adapter.sh — Deterministic failure-adaptation kernel (v8.22.0).
#
# WHY THIS EXISTS
#
# Pre-v8.22.0, the orchestrator's adaptive-failure logic was a markdown table
# in agents/evolve-orchestrator.md. The orchestrator (LLM) would read prior
# failedApproaches entries and "decide" the right action by interpreting the
# table. This was: (1) non-deterministic (LLM may misread), (2) non-testable,
# (3) not phase-gate-enforceable, and (4) too coarse — the "3+ failures of
# any kind → BLOCKED-SYSTEMIC" rule conflated environmental and code issues.
#
# v8.22.0 moves the decision logic into this deterministic shell script.
# Given the cycle's workspace + state.json, this script computes the next
# action and emits it as JSON. The orchestrator subagent reads the JSON and
# follows it verbatim — no longer interpreting markdown rules.
#
# Usage:
#   bash scripts/failure-adapter.sh decide --cycle <N> --workspace <path>
#       Compute the action for the given cycle. Emits JSON to stdout.
#
#   bash scripts/failure-adapter.sh decide --state <path>
#       Test mode: read failedApproaches from a custom state.json path.
#
# Output (stdout): JSON with these fields:
#   {
#     "action": "PROCEED | RETRY-WITH-FALLBACK | BLOCK-CODE | BLOCK-OPERATOR-ACTION",
#     "reason": "human-readable explanation",
#     "remediation": "optional remediation hint (only set for BLOCK-* actions)",
#     "set_env": {...},                    # env vars to set before continuing
#     "skip_phases": [...],                # phases to skip (BLOCK-CODE may skip audit, etc.)
#     "verdict_for_block": "BLOCKED-* | null",  # specific verdict to record on BLOCK
#     "evidence": {                        # for forensic / debugging
#       "non_expired_count": N,
#       "by_class": {...},
#       "consecutive_infra_transient_streak": N
#     }
#   }
#
# Exit codes:
#   0  — decision computed (regardless of action)
#   1  — argument error / state file missing
# 127  — required binary missing (jq)

set -uo pipefail

# v8.18.0: dual-root.
__rr_self="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$__rr_self/resolve-roots.sh"
. "$__rr_self/failure-classifications.sh"
unset __rr_self

STATE_DEFAULT="$EVOLVE_PROJECT_ROOT/.evolve/state.json"

log()  { echo "[failure-adapter] $*" >&2; }
fail() { log "FAIL: $*"; exit 1; }

# ---- Args -----------------------------------------------------------------

CMD="${1:-}"
shift || true
[ "$CMD" = "decide" ] || { echo "usage: failure-adapter.sh decide [--cycle N] [--workspace path] [--state path]" >&2; exit 1; }

CYCLE=""
WORKSPACE=""
STATE_PATH="$STATE_DEFAULT"
while [ $# -gt 0 ]; do
    case "$1" in
        --cycle)     shift; CYCLE="$1" ;;
        --workspace) shift; WORKSPACE="$1" ;;
        --state)     shift; STATE_PATH="$1" ;;
        *) fail "unknown arg: $1" ;;
    esac
    shift
done

command -v jq >/dev/null 2>&1 || { log "missing required binary: jq"; exit 127; }

# ---- Load + auto-prune ----------------------------------------------------

if [ ! -f "$STATE_PATH" ]; then
    # No state file → no failure history → PROCEED.
    jq -nc '{
        action: "PROCEED",
        reason: "no state.json present (first cycle / fresh checkout)",
        set_env: {},
        skip_phases: [],
        verdict_for_block: null,
        evidence: {non_expired_count: 0, by_class: {}, consecutive_infra_transient_streak: 0}
    }'
    exit 0
fi

# Auto-prune expired entries before reading. This makes the decision robust
# to stale state without requiring a separate manual prune step.
bash "$EVOLVE_PLUGIN_ROOT/scripts/cycle-state.sh" prune-expired-failures "$STATE_PATH" 2>/dev/null || true

# ---- Compute features from non-expired entries ----------------------------

NOW_S=$(date -u +%s)

# Filter out expired entries one more time defensively, then compute features.
ENTRIES=$(jq -c --argjson now "$NOW_S" \
    '(.failedApproaches // [])
     | map(select(
         (.expiresAt // "") == "" or
         (.expiresAt | (try fromdateiso8601 catch ($now + 1))) > $now
       ))' "$STATE_PATH")

NON_EXPIRED_COUNT=$(echo "$ENTRIES" | jq 'length')
BY_CLASS=$(echo "$ENTRIES" | jq -c 'group_by(.classification) | map({(.[0].classification // "unknown-classification"): length}) | add // {}')

# Consecutive infrastructure-transient streak (from the END of the array).
# We walk entries in reverse order until we hit a non-infra-transient.
INFRA_STREAK=$(echo "$ENTRIES" | jq -r '
    [.[]
     | select(.classification == "infrastructure-transient" or .classification == "infrastructure")]
    | length' )
# Also separately: count consecutive at the tail.
INFRA_TAIL_STREAK=$(echo "$ENTRIES" | jq -r '
    reverse
    | reduce .[] as $e (
        {streak: 0, broken: false};
        if .broken then .
        elif ($e.classification // "") == "infrastructure-transient" or ($e.classification // "") == "infrastructure"
        then .streak += 1
        else .broken = true
        end
      )
    | .streak')

# Helper: count of a specific classification.
count_class() {
    echo "$ENTRIES" | jq --arg c "$1" 'map(select(.classification == $c)) | length'
}

CODE_AUDIT_FAIL_COUNT=$(count_class "code-audit-fail")
CODE_BUILD_FAIL_COUNT=$(count_class "code-build-fail")
INTENT_REJECTED_COUNT=$(count_class "intent-rejected")
SYSTEMIC_COUNT=$(count_class "infrastructure-systemic")

# ---- Decide ---------------------------------------------------------------

emit() {
    local action="$1" reason="$2" remediation="${3:-}" verdict="${4:-null}"
    # Bash default-value syntax with `{}`/`[]` literals is fragile; expand
    # explicitly to avoid `${var:-{\}}` quirks across shell versions.
    local set_env="${5:-}"; [ -z "$set_env" ] && set_env='{}'
    local skip="${6:-}";    [ -z "$skip" ]    && skip='[]'
    local verdict_arg
    if [ "$verdict" = "null" ] || [ -z "$verdict" ]; then
        verdict_arg='null'
    else
        verdict_arg="\"$verdict\""
    fi
    jq -nc \
        --arg action "$action" \
        --arg reason "$reason" \
        --arg remediation "$remediation" \
        --argjson set_env "$set_env" \
        --argjson skip "$skip" \
        --argjson verdict_for_block "$verdict_arg" \
        --argjson non_expired_count "$NON_EXPIRED_COUNT" \
        --argjson by_class "$BY_CLASS" \
        --argjson tail_streak "$INFRA_TAIL_STREAK" \
        '{
            action: $action,
            reason: $reason,
            remediation: $remediation,
            set_env: $set_env,
            skip_phases: $skip,
            verdict_for_block: $verdict_for_block,
            evidence: {
                non_expired_count: $non_expired_count,
                by_class: $by_class,
                consecutive_infra_transient_streak: $tail_streak
            }
        }'
}

# Decision tree (priority order — first match wins):
#
# 1. intent-rejected (any non-expired) → BLOCK-CODE, SCOPE-REJECTED.
#    User goal is out of scope; loop cannot proceed without operator refinement.
if [ "$INTENT_REJECTED_COUNT" -gt 0 ]; then
    emit "BLOCK-CODE" \
        "$INTENT_REJECTED_COUNT prior intent-rejected (out-of-scope IBTC)" \
        "Refine the goal description to be in-scope, then re-run /evolve-loop." \
        "SCOPE-REJECTED"
    exit 0
fi

# 2. infrastructure-systemic (any non-expired) → BLOCK-OPERATOR-ACTION.
#    Tooling-missing / host-broken — operator must intervene.
if [ "$SYSTEMIC_COUNT" -gt 0 ]; then
    last_systemic_summary=$(echo "$ENTRIES" | jq -r '
        map(select(.classification == "infrastructure-systemic")) | last | .summary // "(no summary)"' \
        | head -c 200)
    emit "BLOCK-OPERATOR-ACTION" \
        "$SYSTEMIC_COUNT non-expired infrastructure-systemic failure(s); last summary: $last_systemic_summary" \
        "Investigate the systemic infrastructure issue (tooling, host, claude-cli). Use scripts/state-prune.sh --classification infrastructure-systemic after fixing." \
        "BLOCKED-SYSTEMIC"
    exit 0
fi

# 3. 2+ code-audit-fail → BLOCK-CODE, BLOCKED-RECURRING-AUDIT-FAIL.
if [ "$CODE_AUDIT_FAIL_COUNT" -ge 2 ]; then
    emit "BLOCK-CODE" \
        "$CODE_AUDIT_FAIL_COUNT non-expired code-audit-fail entries (within 30d retention)" \
        "Auditor has rejected code N times. Pick a materially different task or prune via scripts/state-prune.sh --classification code-audit-fail after addressing root cause." \
        "BLOCKED-RECURRING-AUDIT-FAIL"
    exit 0
fi

# 4. 2+ code-build-fail → BLOCK-CODE, BLOCKED-RECURRING-BUILD-FAIL.
if [ "$CODE_BUILD_FAIL_COUNT" -ge 2 ]; then
    emit "BLOCK-CODE" \
        "$CODE_BUILD_FAIL_COUNT non-expired code-build-fail entries (within 30d retention)" \
        "Builder has failed to compile/test N times. Pick a materially different task or prune via scripts/state-prune.sh --classification code-build-fail." \
        "BLOCKED-RECURRING-BUILD-FAIL"
    exit 0
fi

# 5. 3+ consecutive infrastructure-transient (TAIL streak) → BLOCK-OPERATOR-ACTION.
#    The fallback flag isn't unblocking the cycle. Operator must investigate
#    nested-claude or run from non-sandboxed shell.
if [ "$INFRA_TAIL_STREAK" -ge 3 ]; then
    emit "BLOCK-OPERATOR-ACTION" \
        "$INFRA_TAIL_STREAK consecutive infrastructure-transient failures despite EPERM-fallback. The kernel sandbox-exec is being rejected at every retry." \
        "Either: (1) run /evolve-loop from a non-sandboxed terminal, OR (2) run scripts/state-prune.sh --classification infrastructure-transient after confirming the underlying issue is resolved, OR (3) file an issue with cycle ledger entry." \
        "BLOCKED-SYSTEMIC"
    exit 0
fi

# 6. 1+ infrastructure-transient → RETRY-WITH-FALLBACK.
#    Auto-enable the EPERM fallback (defense in depth — dispatcher already
#    sets it for nested-claude, this is belt-and-suspenders).
INFRA_T_COUNT=$(count_class "infrastructure-transient")
if [ "$INFRA_T_COUNT" -gt 0 ]; then
    emit "RETRY-WITH-FALLBACK" \
        "$INFRA_T_COUNT prior infrastructure-transient (within 1d retention); attempting with EPERM fallback enabled" \
        "" \
        "null" \
        '{"EVOLVE_SANDBOX_FALLBACK_ON_EPERM":"1"}'
    exit 0
fi

# 7. Default: PROCEED (no concerning failure history).
emit "PROCEED" \
    "no recent failures requiring adaptation (non-expired count=$NON_EXPIRED_COUNT)" \
    "" \
    "null"
exit 0
