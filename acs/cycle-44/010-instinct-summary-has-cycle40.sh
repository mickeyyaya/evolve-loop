#!/usr/bin/env bash
# AC-ID:         cycle-44-010
# Description:   state.json:instinctSummary[] contains >= 1 entry with id starting "cycle-40"
# Evidence:      .evolve/state.json (instinctSummary[].id)
# Author:        builder
# Created:       2026-05-14T00:00:00Z
# Acceptance-of: build-report.md T2 (A7)
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || echo ".")"
__rr_self="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [ -f "$__rr_self/../../scripts/lifecycle/resolve-roots.sh" ]; then
    . "$__rr_self/../../scripts/lifecycle/resolve-roots.sh" 2>/dev/null || true
fi
PROJECT_ROOT="${EVOLVE_PROJECT_ROOT:-$REPO_ROOT}"
OUTPUT_DIR="$PROJECT_ROOT/.evolve/runs/cycle-44/acs-output"
mkdir -p "$OUTPUT_DIR"

STATE="$PROJECT_ROOT/.evolve/state.json"

if [ ! -f "$STATE" ]; then
    echo "FAIL: state.json not found: $STATE" | tee "$OUTPUT_DIR/010-result.txt"
    exit 1
fi

command -v jq >/dev/null 2>&1 || { echo "FAIL: jq required" | tee "$OUTPUT_DIR/010-result.txt"; exit 1; }

count=$(jq '[.instinctSummary[]?.id // "" | select(startswith("cycle-40"))] | length' "$STATE" 2>/dev/null || echo "0")

if [ "$count" -lt 1 ]; then
    echo "FAIL: no instinctSummary entries with id starting 'cycle-40' found in state.json (count=$count). Run: bash scripts/utility/backfill-lessons.sh --cycle 40" | tee "$OUTPUT_DIR/010-result.txt"
    exit 1
fi

echo "PASS: $count instinctSummary entry(ies) with id starting 'cycle-40' in state.json" | tee "$OUTPUT_DIR/010-result.txt"
exit 0
