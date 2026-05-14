#!/usr/bin/env bash
# AC-ID:         cycle-43-003
# Description:   phase-gate.sh contains gate_retrospective_to_complete() function and dispatch entry
# Evidence:      scripts/lifecycle/phase-gate.sh
# Author:        builder
# Created:       2026-05-14T00:00:00Z
# Acceptance-of: build-report.md T3-c (A3)
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || echo ".")"
FILE="$REPO_ROOT/scripts/lifecycle/phase-gate.sh"

[ -f "$FILE" ] || { echo "FAIL: phase-gate.sh not found"; exit 1; }

# Function must exist
grep -q "^gate_retrospective_to_complete()" "$FILE" || { echo "FAIL: gate_retrospective_to_complete() function not found"; exit 1; }

# Dispatch entry must exist
grep -q "retrospective-to-complete) gate_retrospective_to_complete" "$FILE" || { echo "FAIL: dispatch entry for retrospective-to-complete not found"; exit 1; }

# Must verify lessonIds against on-disk YAMLs (INTEGRITY check)
grep -q "lessonIds\|INTEGRITY_FAIL\|lesson.*yaml\|yaml.*lesson" "$FILE" || { echo "FAIL: YAML integrity check not found in gate_retrospective_to_complete"; exit 1; }

echo "PASS: phase-gate.sh has gate_retrospective_to_complete() with YAML integrity check and dispatch entry"
exit 0
