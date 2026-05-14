#!/usr/bin/env bash
# AC-ID:         cycle-44-006
# Description:   scripts/cli_adapters/claude.sh contains --effort flag dispatch (P-NEW-26)
# Evidence:      scripts/cli_adapters/claude.sh (EFFORT_LEVEL read + CMD+= block)
# Author:        builder
# Created:       2026-05-14T00:00:00Z
# Acceptance-of: build-report.md T1 (A1)
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || echo ".")"
OUTPUT_DIR="$REPO_ROOT/.evolve/runs/cycle-44/acs-output"
mkdir -p "$OUTPUT_DIR"

FILE="$REPO_ROOT/scripts/cli_adapters/claude.sh"

if [ ! -f "$FILE" ]; then
    echo "FAIL: scripts/cli_adapters/claude.sh not found" | tee "$OUTPUT_DIR/006-result.txt"
    exit 1
fi

# Check --effort flag is dispatched
if ! grep -q -- '--effort' "$FILE"; then
    echo "FAIL: '--effort' flag not found in scripts/cli_adapters/claude.sh" | tee "$OUTPUT_DIR/006-result.txt"
    exit 1
fi

# Check effort_level is read from profile
if ! grep -q 'effort_level' "$FILE"; then
    echo "FAIL: effort_level field not read from profile in scripts/cli_adapters/claude.sh" | tee "$OUTPUT_DIR/006-result.txt"
    exit 1
fi

echo "PASS: --effort flag and effort_level read wired in scripts/cli_adapters/claude.sh" | tee "$OUTPUT_DIR/006-result.txt"
exit 0
