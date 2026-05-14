#!/usr/bin/env bash
# AC6: grep -c "context_compact" profiles/builder.json returns 0
# metadata: cycle=46 task=T2b ac=AC6 risk=low

set -uo pipefail

PROJECT_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
PROFILE="$PROJECT_ROOT/.evolve/profiles/builder.json"

count=$(grep -c "context_compact" "$PROFILE" 2>/dev/null || echo "0")
if [ "$count" -ne 0 ]; then
    echo "FAIL: builder.json still contains $count 'context_compact' field(s)" >&2
    grep "context_compact" "$PROFILE" >&2
    exit 1
fi

# Verify it's still valid JSON
if ! jq empty "$PROFILE" 2>/dev/null; then
    echo "FAIL: builder.json is not valid JSON after edit" >&2
    exit 1
fi

echo "PASS: AC6 — builder.json has no context_compact fields (phantom fields removed)"
exit 0
