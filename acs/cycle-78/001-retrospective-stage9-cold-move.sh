#!/bin/bash
# ACS Cycle-78 Stage 9: retrospective cold-move + scout ceiling calibration
set -uo pipefail

RETRO="agents/evolve-retrospective.md"
REFERENCE="agents/evolve-retrospective-reference.md"
ADR="docs/architecture/adr/0016-retrospective-cold-move-stage9.md"
SCOUT_PROFILE=".evolve/profiles/scout.json"

# AC1: line count reduction >= 10% (281 -> <= 253)
line_count=$(wc -l < "$RETRO")
if [ "$line_count" -gt 253 ]; then
    echo "FAIL AC1: $RETRO has $line_count lines (expected <= 253)" >&2
    exit 1
fi

# AC2: digest-format-template section in reference doc
if ! grep -q '## Section: digest-format-template' "$REFERENCE"; then
    echo "FAIL AC2: '## Section: digest-format-template' not found in $REFERENCE" >&2
    exit 1
fi

# AC3: handoff-schema section in reference doc
if ! grep -q '## Section: handoff-schema' "$REFERENCE"; then
    echo "FAIL AC3: '## Section: handoff-schema' not found in $REFERENCE" >&2
    exit 1
fi

# AC4: pointer to digest-format-template in hot persona
if ! grep -qE 'evolve-retrospective-reference\.md.*digest-format-template' "$RETRO"; then
    echo "FAIL AC4: pointer to digest-format-template not found in $RETRO" >&2
    exit 1
fi

# AC5: pointer to handoff-schema in hot persona
if ! grep -qE 'evolve-retrospective-reference\.md.*handoff-schema' "$RETRO"; then
    echo "FAIL AC5: pointer to handoff-schema not found in $RETRO" >&2
    exit 1
fi

# AC6: ADR-0016 exists
if [ ! -f "$ADR" ]; then
    echo "FAIL AC6: $ADR not found" >&2
    exit 1
fi

# AC7: scout max_turns == 30
scout_turns=$(grep '"max_turns"' "$SCOUT_PROFILE" | grep -o '[0-9]*')
if [ "$scout_turns" != "30" ]; then
    echo "FAIL AC7: scout max_turns is $scout_turns (expected 30)" >&2
    exit 1
fi

echo "PASS: AC1 (lines=$line_count <= 253), AC2 (digest-format-template in reference), AC3 (handoff-schema in reference), AC4 (pointer in persona), AC5 (handoff pointer in persona), AC6 (ADR-0016 exists), AC7 (scout max_turns=30)"
