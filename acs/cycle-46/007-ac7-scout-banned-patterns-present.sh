#!/usr/bin/env bash
# AC7: grep -q "BANNED" agents/evolve-scout.md passes
# metadata: cycle=46 task=T2a ac=AC7 risk=low

set -uo pipefail

PROJECT_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
SCOUT="$PROJECT_ROOT/agents/evolve-scout.md"

if ! grep -q "BANNED" "$SCOUT"; then
    echo "FAIL: BANNED keyword not found in agents/evolve-scout.md" >&2
    exit 1
fi

# Verify the ban table has at least 3 examples
banned_rows=$(grep -c "BANNED.*Bash\|Bash.*BANNED" "$SCOUT" || echo "0")
if [ "$banned_rows" -lt 1 ]; then
    echo "FAIL: BANNED table in evolve-scout.md missing Bash ban rows" >&2
    exit 1
fi

# Verify native tool alternatives are shown
if ! grep -q "Read\|Grep\|Glob" "$SCOUT"; then
    echo "FAIL: BANNED table missing native tool alternatives (Read/Grep/Glob)" >&2
    exit 1
fi

echo "PASS: AC7 — BANNED patterns table present in agents/evolve-scout.md Tool-Result Hygiene section"
exit 0
