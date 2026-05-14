#!/usr/bin/env bash
# AC5: gate_audit_to_retrospective fires retro on non-empty abnormal-events.jsonl (PASS cycles)
# metadata: cycle=46 task=T1-phase-b ac=AC5 risk=medium

set -uo pipefail

PROJECT_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
GATE="$PROJECT_ROOT/scripts/lifecycle/phase-gate.sh"

# Verify the gate checks abnormal-events.jsonl
if ! grep -A50 "gate_audit_to_retrospective()" "$GATE" | grep -q "abnormal-events.jsonl"; then
    echo "FAIL: gate_audit_to_retrospective does not check abnormal-events.jsonl" >&2
    exit 1
fi

# Verify it allows PASS+abnormal-events to proceed to retro (has the -s check for non-empty)
if ! grep -A50 "gate_audit_to_retrospective()" "$GATE" | grep -q "\-s.*abnormal-events"; then
    echo "FAIL: gate_audit_to_retrospective does not use -s (non-empty) check on abnormal-events.jsonl" >&2
    exit 1
fi

# Verify PASS cycles with abnormal events return 0 (allow retro)
if ! grep -A50 "gate_audit_to_retrospective()" "$GATE" | grep -q "PASS-WITH-ABNORMAL\|return 0"; then
    echo "FAIL: gate_audit_to_retrospective does not return 0 for PASS+abnormal-events case" >&2
    exit 1
fi

echo "PASS: AC5 — gate_audit_to_retrospective fires retro on non-empty abnormal-events.jsonl"
exit 0
