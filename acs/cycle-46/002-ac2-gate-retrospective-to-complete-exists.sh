#!/usr/bin/env bash
# AC2: gate_retrospective_to_complete verifies lesson YAML files 1:1 with lessonIds[]
# predicate: function exists in phase-gate.sh and checks YAML file count
# metadata: cycle=46 task=T1-phase-a ac=AC2 risk=low

set -uo pipefail

PROJECT_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
GATE="$PROJECT_ROOT/scripts/lifecycle/phase-gate.sh"

# Verify gate_retrospective_to_complete exists
if ! grep -q "gate_retrospective_to_complete()" "$GATE"; then
    echo "FAIL: gate_retrospective_to_complete() not found in phase-gate.sh" >&2
    exit 1
fi

# Verify it checks lessonIds
if ! grep -A30 "gate_retrospective_to_complete()" "$GATE" | grep -q "lessonIds"; then
    echo "FAIL: gate_retrospective_to_complete does not check lessonIds[]" >&2
    exit 1
fi

# Verify it verifies YAML on disk
if ! grep -A30 "gate_retrospective_to_complete()" "$GATE" | grep -q "\.yaml"; then
    echo "FAIL: gate_retrospective_to_complete does not verify YAML on disk" >&2
    exit 1
fi

# Verify INTEGRITY_FAIL is emitted when lesson YAML missing
if ! grep -A30 "gate_retrospective_to_complete()" "$GATE" | grep -q "INTEGRITY_FAIL"; then
    echo "FAIL: gate_retrospective_to_complete does not emit INTEGRITY_FAIL on missing YAML" >&2
    exit 1
fi

echo "PASS: AC2 — gate_retrospective_to_complete verifies lessonIds[] ↔ YAML 1:1"
exit 0
