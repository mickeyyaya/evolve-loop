#!/usr/bin/env bash
# AC-ID:         cycle-43-001
# Description:   evolve-retrospective.md contains MUST-FIRST YAML-write contract
# Evidence:      agents/evolve-retrospective.md (Step 5 + Final checks)
# Author:        builder
# Created:       2026-05-14T00:00:00Z
# Acceptance-of: build-report.md T3-a (A1)
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || echo ".")"
FILE="$REPO_ROOT/agents/evolve-retrospective.md"

[ -f "$FILE" ] || { echo "FAIL: evolve-retrospective.md not found"; exit 1; }

# Must contain MUST-FIRST language in Step 5
grep -q "MUST-FIRST" "$FILE" || { echo "FAIL: MUST-FIRST contract not found in evolve-retrospective.md"; exit 1; }

# Must contain INTEGRITY_FAIL reference (exit 2 on missing YAML)
grep -q "INTEGRITY_FAIL" "$FILE" || { echo "FAIL: INTEGRITY_FAIL exit 2 contract not found"; exit 1; }

# Final checks section must reference dangling IDs exit 2
grep -q "dangling IDs\|exit 2" "$FILE" || { echo "FAIL: exit 2 on dangling IDs not found in Final checks"; exit 1; }

echo "PASS: evolve-retrospective.md has MUST-FIRST YAML-write contract with INTEGRITY_FAIL handling"
exit 0
