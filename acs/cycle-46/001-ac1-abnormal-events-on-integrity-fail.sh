#!/usr/bin/env bash
# AC1: abnormal-events.jsonl written when merge-lesson-into-state.sh exits non-zero (INTEGRITY_FAIL)
# predicate: merge-lesson-into-state.sh exit-2 path appends to abnormal-events.jsonl
# metadata: cycle=46 task=T1-phase-b ac=AC1 risk=medium

set -uo pipefail

PROJECT_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
SCRIPT="$PROJECT_ROOT/scripts/failure/merge-lesson-into-state.sh"

# Verify the script exists and contains the persistence-fail event type
if ! grep -q "persistence-fail" "$SCRIPT"; then
    echo "FAIL: merge-lesson-into-state.sh does not contain persistence-fail event type" >&2
    exit 1
fi

# Verify the _append_abnormal_event function is defined in the script
if ! grep -q "_append_abnormal_event" "$SCRIPT"; then
    echo "FAIL: merge-lesson-into-state.sh missing _append_abnormal_event function" >&2
    exit 1
fi

# Verify the event is appended before exit 2 in the YAML-missing branch
if ! grep -A3 "INTEGRITY-FAIL: lesson" "$SCRIPT" | grep -q "_append_abnormal_event"; then
    echo "FAIL: _append_abnormal_event not called before exit 2 in INTEGRITY-FAIL branch" >&2
    exit 1
fi

echo "PASS: AC1 — merge-lesson-into-state.sh wires abnormal event on persistence-fail exit 2"
exit 0
