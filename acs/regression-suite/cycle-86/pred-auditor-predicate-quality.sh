#!/usr/bin/env bash
# Predicate: agents/evolve-auditor.md contains 'Predicate quality review' section
# Behavioral: reads file, counts lines in section to verify substantive content
set -uo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
AUDITOR_FILE="$REPO_ROOT/agents/evolve-auditor.md"
[ -f "$AUDITOR_FILE" ] || { echo "MISSING: $AUDITOR_FILE" >&2; exit 1; }
# Extract section content: start after the header line, stop at next ## section
section_lines=$(awk '
    found && /^## / { exit }
    /^## Predicate quality review/ { found=1; next }
    found { print }
' "$AUDITOR_FILE" | wc -l | tr -d ' \n')
# Section must have at least 5 lines of substantive content
[ "${section_lines:-0}" -ge 5 ]
