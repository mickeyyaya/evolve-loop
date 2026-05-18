#!/usr/bin/env bash
# AC-ID: cycle-84-001
# Verify lint-markdown-structure-baseline.txt exists and is non-empty (>=10 lines).
set -uo pipefail
REPO_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
BASELINE="$REPO_ROOT/.evolve/baselines/lint-markdown-structure-baseline.txt"
[ -f "$BASELINE" ] || { echo "RED cycle-84-001: $BASELINE missing"; exit 1; }
line_count=$(wc -l < "$BASELINE" | tr -d ' ')
if [ "$line_count" -lt 10 ]; then
    echo "RED cycle-84-001: baseline has $line_count lines (need >=10)"
    exit 1
fi
echo "GREEN cycle-84-001: lint-markdown-structure-baseline.txt exists ($line_count lines)"
exit 0
