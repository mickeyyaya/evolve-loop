#!/usr/bin/env bash
# AC-ID:         cycle-44-009
# Description:   >= 2 cycle-40-*.yaml lesson files exist in .evolve/instincts/lessons/
# Evidence:      .evolve/instincts/lessons/cycle-40-*.yaml
# Author:        builder
# Created:       2026-05-14T00:00:00Z
# Acceptance-of: build-report.md T2 (A6)
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || echo ".")"
__rr_self="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [ -f "$__rr_self/../../scripts/lifecycle/resolve-roots.sh" ]; then
    . "$__rr_self/../../scripts/lifecycle/resolve-roots.sh" 2>/dev/null || true
fi
PROJECT_ROOT="${EVOLVE_PROJECT_ROOT:-$REPO_ROOT}"
OUTPUT_DIR="$PROJECT_ROOT/.evolve/runs/cycle-44/acs-output"
mkdir -p "$OUTPUT_DIR"

LESSONS_DIR="$PROJECT_ROOT/.evolve/instincts/lessons"

if [ ! -d "$LESSONS_DIR" ]; then
    echo "FAIL: lessons dir not found: $LESSONS_DIR" | tee "$OUTPUT_DIR/009-result.txt"
    exit 1
fi

count=0
for f in "$LESSONS_DIR"/cycle-40-*.yaml; do
    [ -f "$f" ] && count=$((count + 1))
done

if [ "$count" -lt 2 ]; then
    echo "FAIL: found $count cycle-40-*.yaml files (need >= 2). Run: bash scripts/utility/backfill-lessons.sh --cycle 40" | tee "$OUTPUT_DIR/009-result.txt"
    exit 1
fi

echo "PASS: found $count cycle-40-*.yaml lesson files in $LESSONS_DIR" | tee "$OUTPUT_DIR/009-result.txt"
exit 0
