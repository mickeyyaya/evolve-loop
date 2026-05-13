#!/usr/bin/env bash
# detect-tree-sha-breach.sh — Scan ship-binding.json sidecars for tree-SHA mismatches.
#
# Covers all cycles from C1 (cycle 32) onwards where ship-binding.json was written.
# Pre-C1 cycles have no sidecar — omitted from output (see docs/incidents/cycle-31-c38-orphan.md).
#
# Usage:
#   bash scripts/observability/detect-tree-sha-breach.sh [--json]
#
# Exit codes:
#   0 — no breaches detected (or no sidecar files found)
#   1 — at least one BREACH detected

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
RUNS_DIR="$REPO_ROOT/.evolve/runs"
JSON_OUT=0
[ "${1:-}" = "--json" ] && JSON_OUT=1

BREACH_COUNT=0
TOTAL=0

_json_entries=""

# Print header for tabular mode
if [ "$JSON_OUT" = "0" ]; then
    printf '%-6s %-9s %-42s %-42s %s\n' "CYCLE" "COMMIT" "AUDIT_SHA" "COMMITTED_SHA" "STATUS"
    printf '%-6s %-9s %-42s %-42s %s\n' "------" "---------" "------------------------------------------" "------------------------------------------" "------"
fi

# Glob binding files sorted by cycle number (numeric sort via awk)
BINDINGS=$(ls "$RUNS_DIR"/cycle-*/ship-binding.json 2>/dev/null | awk -F'/' '
{
    for (i=1; i<=NF; i++) {
        if ($i ~ /^cycle-[0-9]+$/) {
            n = $i
            sub(/^cycle-/, "", n)
            print n, $0
        }
    }
}' | sort -n | awk '{print $2}')

if [ -z "$BINDINGS" ]; then
    if [ "$JSON_OUT" = "0" ]; then
        echo "(no ship-binding.json files found — C1 bindings start at cycle 32)"
    else
        echo '{"total":0,"breaches":0,"entries":[]}'
    fi
    exit 0
fi

while IFS= read -r binding_file; do
    [ -f "$binding_file" ] || continue

    # Parse fields via jq if available, else grep/sed fallback
    if command -v jq >/dev/null 2>&1; then
        audit_sha=$(jq -r '.audit_bound_tree_sha // ""' "$binding_file" 2>/dev/null || echo "")
        committed_sha=$(jq -r '.tree_sha_committed // ""' "$binding_file" 2>/dev/null || echo "")
        commit_sha=$(jq -r '.commit_sha // ""' "$binding_file" 2>/dev/null || echo "")
        cycle=$(jq -r '.cycle // ""' "$binding_file" 2>/dev/null || echo "")
    else
        audit_sha=$(grep -o '"audit_bound_tree_sha":"[^"]*"' "$binding_file" 2>/dev/null | sed 's/.*:"\([^"]*\)".*/\1/' || echo "")
        committed_sha=$(grep -o '"tree_sha_committed":"[^"]*"' "$binding_file" 2>/dev/null | sed 's/.*:"\([^"]*\)".*/\1/' || echo "")
        commit_sha=$(grep -o '"commit_sha":"[^"]*"' "$binding_file" 2>/dev/null | sed 's/.*:"\([^"]*\)".*/\1/' || echo "")
        cycle=$(grep -o '"cycle":[0-9]*' "$binding_file" 2>/dev/null | sed 's/.*:\([0-9]*\).*/\1/' || echo "")
    fi

    TOTAL=$((TOTAL + 1))
    commit_short="${commit_sha:0:8}"

    if [ -z "$audit_sha" ] || [ -z "$committed_sha" ]; then
        status="MISSING-SHA"
    elif [ "$audit_sha" = "$committed_sha" ]; then
        status="OK"
    else
        status="BREACH"
        BREACH_COUNT=$((BREACH_COUNT + 1))
    fi

    if [ "$JSON_OUT" = "0" ]; then
        printf '%-6s %-9s %-42s %-42s %s\n' \
            "${cycle:-?}" "${commit_short:-?}" \
            "${audit_sha:-(empty)}" "${committed_sha:-(empty)}" "$status"
    else
        _entry="{\"cycle\":${cycle:-0},\"commit_sha\":\"${commit_sha:-}\",\"audit_bound_tree_sha\":\"${audit_sha:-}\",\"tree_sha_committed\":\"${committed_sha:-}\",\"status\":\"$status\"}"
        _json_entries="${_json_entries}${_json_entries:+,}${_entry}"
    fi
done <<EOF
$BINDINGS
EOF

if [ "$JSON_OUT" = "0" ]; then
    echo ""
    echo "Total: $TOTAL  Breaches: $BREACH_COUNT"
else
    echo "{\"total\":$TOTAL,\"breaches\":$BREACH_COUNT,\"entries\":[${_json_entries}]}"
fi

[ "$BREACH_COUNT" = "0" ]
