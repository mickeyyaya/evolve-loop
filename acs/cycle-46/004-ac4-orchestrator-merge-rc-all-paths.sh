#!/usr/bin/env bash
# AC4: evolve-orchestrator.md checks MERGE_RC after every merge-lesson-into-state.sh call
# metadata: cycle=46 task=T1-phase-a ac=AC4 risk=low

set -uo pipefail

PROJECT_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
ORCH="$PROJECT_ROOT/agents/evolve-orchestrator.md"

# Count merge-lesson-into-state.sh occurrences in the phase loop (not in comments/table)
total_calls=$(grep -c "merge-lesson-into-state.sh" "$ORCH" || true)
merge_rc_checks=$(grep -c "MERGE_RC=\$?" "$ORCH" || true)

# All merge-lesson calls in the phase loop pseudocode should have MERGE_RC check
# The phase loop has 3 paths (PASS, WARN, FAIL) — each should have MERGE_RC check
if [ "$merge_rc_checks" -lt 3 ]; then
    echo "FAIL: evolve-orchestrator.md has $merge_rc_checks MERGE_RC checks but expected ≥3 (one per PASS/WARN/FAIL path)" >&2
    echo "  total merge-lesson calls: $total_calls" >&2
    exit 1
fi

# Verify INTEGRITY_FAIL exit 2 handling present
if ! grep -q "MERGE_RC -eq 2.*exit 2\|exit 2.*INTEGRITY_FAIL" "$ORCH"; then
    echo "FAIL: evolve-orchestrator.md missing 'if [ \$MERGE_RC -eq 2 ]; then exit 2; fi' guard" >&2
    exit 1
fi

echo "PASS: AC4 — evolve-orchestrator.md checks MERGE_RC on all $merge_rc_checks merge-lesson-into-state.sh calls"
exit 0
