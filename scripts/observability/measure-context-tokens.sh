#!/usr/bin/env bash
#
# measure-context-tokens.sh — measure per-phase prompt token footprint for a cycle.
#
# v8.61.0 Campaign A Cycle A3.
#
# Different from token-profiler.sh (static catalog of skill/agent/phase files):
# this script measures the ACTUAL prompt material assembled for each phase in
# a specific cycle — bedrock (role-specific from build-invocation-context.sh)
# + role-context-builder output (orchestrator-prompt + per-phase prompts) +
# task envelope. Used to verify Campaign A/B/C/D token-floor reduction targets.
#
# Token estimate: 1 token ≈ 4 bytes for English text (Anthropic's published
# upper bound). Same heuristic role-context-builder.sh:205 uses.
#
# Usage:
#   bash measure-context-tokens.sh <cycle>
#   bash measure-context-tokens.sh <cycle> --json    # machine-readable
#
# Exit codes:
#   0 — measurements emitted
#   1 — cycle workspace not found
#   2 — missing or invalid cycle argument

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CYCLE="${1:-}"
JSON_OUTPUT=0
[ "${2:-}" = "--json" ] && JSON_OUTPUT=1

if [ -z "$CYCLE" ] || ! [[ "$CYCLE" =~ ^[0-9]+$ ]]; then
    echo "usage: measure-context-tokens.sh <cycle-number> [--json]" >&2
    exit 2
fi

WORKSPACE="$REPO_ROOT/.evolve/runs/cycle-$CYCLE"
if [ ! -d "$WORKSPACE" ]; then
    echo "cycle workspace not found: $WORKSPACE" >&2
    exit 1
fi

BIC="$REPO_ROOT/scripts/dispatch/build-invocation-context.sh"

# Compute bytes + token estimate for a file (returns "0 0" if missing).
_size() {
    if [ -f "$1" ]; then
        local b
        b=$(wc -c < "$1" | tr -d ' ')
        echo "$b $((b / 4))"
    else
        echo "0 0"
    fi
}

# Bedrock size for a role via build-invocation-context.sh.
_bedrock_size() {
    local role="$1"
    if [ -x "$BIC" ]; then
        local b
        b=$(bash "$BIC" "$role" 2>/dev/null | wc -c | tr -d ' ')
        echo "$b $((b / 4))"
    else
        echo "0 0"
    fi
}

declare -a PHASES=(intent scout triage tdd-engineer plan-reviewer builder auditor retrospective memo)
TOTAL_BYTES=0
TOTAL_TOKENS=0

if [ "$JSON_OUTPUT" = "1" ]; then
    printf '{\n  "cycle": %s,\n  "phases": {\n' "$CYCLE"
    first=1
else
    printf '%-18s %12s %12s %12s %12s\n' "phase" "bedrock_b" "context_b" "total_b" "tokens"
    printf '%-18s %12s %12s %12s %12s\n' "-----" "---------" "---------" "-------" "------"
fi

for phase in "${PHASES[@]}"; do
    bedrock_info=$(_bedrock_size "$phase")
    bedrock_b=$(echo "$bedrock_info" | awk '{print $1}')
    bedrock_t=$(echo "$bedrock_info" | awk '{print $2}')

    # role-context-builder output is typically not persisted as a separate
    # file; the closest proxies are orchestrator-prompt.md (assembled context)
    # and the per-phase report's input. Use orchestrator-prompt for cycle-wide
    # context size.
    context_info=$(_size "$WORKSPACE/orchestrator-prompt.md")
    context_b=$(echo "$context_info" | awk '{print $1}')
    context_t=$(echo "$context_info" | awk '{print $2}')

    # Per-phase report (output side; included for visibility, not added to total).
    report_info=$(_size "$WORKSPACE/${phase}-report.md")

    total_b=$((bedrock_b + context_b))
    total_t=$((bedrock_t + context_t))
    TOTAL_BYTES=$((TOTAL_BYTES + total_b))
    TOTAL_TOKENS=$((TOTAL_TOKENS + total_t))

    if [ "$JSON_OUTPUT" = "1" ]; then
        [ "$first" = "1" ] || printf ',\n'
        printf '    "%s": {"bedrock_bytes": %s, "context_bytes": %s, "total_bytes": %s, "tokens": %s}' \
            "$phase" "$bedrock_b" "$context_b" "$total_b" "$total_t"
        first=0
    else
        printf '%-18s %12s %12s %12s %12s\n' "$phase" "$bedrock_b" "$context_b" "$total_b" "$total_t"
    fi
done

if [ "$JSON_OUTPUT" = "1" ]; then
    printf '\n  },\n  "total": {"bytes": %s, "tokens": %s}\n}\n' "$TOTAL_BYTES" "$TOTAL_TOKENS"
else
    printf '%-18s %12s %12s %12s %12s\n' "-----" "---------" "---------" "-------" "------"
    printf '%-18s %12s %12s %12s %12s\n' "TOTAL" "" "" "$TOTAL_BYTES" "$TOTAL_TOKENS"
fi
