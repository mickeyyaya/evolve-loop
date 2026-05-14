#!/usr/bin/env bash
# AC-ID:         cycle-43-004
# Description:   evolve-auditor.md contains STOP CRITERION section with >=3 completion gates
# Evidence:      agents/evolve-auditor.md (## STOP CRITERION section)
# Author:        builder
# Created:       2026-05-14T00:00:00Z
# Acceptance-of: build-report.md T1 (A1)
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || echo ".")"
FILE="$REPO_ROOT/agents/evolve-auditor.md"

[ -f "$FILE" ] || { echo "FAIL: evolve-auditor.md not found"; exit 1; }

# STOP CRITERION section must exist
grep -q "## STOP CRITERION" "$FILE" || { echo "FAIL: ## STOP CRITERION section not found in evolve-auditor.md"; exit 1; }

# Must have at least 3 named completion gates (count table rows with gate names)
GATE_COUNT=$(grep -c "predicates-run\|verdict-decided\|report-written\|defects-listed" "$FILE" || true)
[ "$GATE_COUNT" -ge 3 ] || { echo "FAIL: fewer than 3 named completion gates found (got $GATE_COUNT)"; exit 1; }

# Must have banned post-report patterns
grep -q "Banned Post-Report\|banned.*post.*report\|post-report" "$FILE" || { echo "FAIL: banned post-report patterns section not found"; exit 1; }

echo "PASS: evolve-auditor.md has STOP CRITERION with >=3 completion gates and banned post-report patterns"
exit 0
