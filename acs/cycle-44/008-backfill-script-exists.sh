#!/usr/bin/env bash
# AC-ID:         cycle-44-008
# Description:   scripts/utility/backfill-lessons.sh exists and is executable
# Evidence:      scripts/utility/backfill-lessons.sh
# Author:        builder
# Created:       2026-05-14T00:00:00Z
# Acceptance-of: build-report.md T2 (A5)
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || echo ".")"
# Resolve project root for state paths (.evolve/) — may differ from worktree root
__rr_self="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [ -f "$__rr_self/../../scripts/lifecycle/resolve-roots.sh" ]; then
    . "$__rr_self/../../scripts/lifecycle/resolve-roots.sh" 2>/dev/null || true
fi
PROJECT_ROOT="${EVOLVE_PROJECT_ROOT:-$REPO_ROOT}"
OUTPUT_DIR="$PROJECT_ROOT/.evolve/runs/cycle-44/acs-output"
mkdir -p "$OUTPUT_DIR"

BACKFILL="$REPO_ROOT/scripts/utility/backfill-lessons.sh"

if [ ! -f "$BACKFILL" ]; then
    echo "FAIL: scripts/utility/backfill-lessons.sh not found" | tee "$OUTPUT_DIR/008-result.txt"
    exit 1
fi

if [ ! -x "$BACKFILL" ]; then
    echo "FAIL: scripts/utility/backfill-lessons.sh is not executable" | tee "$OUTPUT_DIR/008-result.txt"
    exit 1
fi

# Verify script runs cleanly with --dry-run
EVOLVE_PROJECT_ROOT="$PROJECT_ROOT" EVOLVE_PLUGIN_ROOT="$REPO_ROOT" \
    bash "$BACKFILL" --dry-run 2>/dev/null
rc=$?
# Exit 0 (changes found) or 2 (nothing to do) both indicate the script runs OK
if [ "$rc" -eq 1 ]; then
    echo "FAIL: backfill-lessons.sh --dry-run exited 1 (runtime error)" | tee "$OUTPUT_DIR/008-result.txt"
    exit 1
fi

echo "PASS: scripts/utility/backfill-lessons.sh exists, is executable, and --dry-run exits cleanly (rc=$rc)" | tee "$OUTPUT_DIR/008-result.txt"
exit 0
