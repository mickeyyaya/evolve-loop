#!/bin/bash
# ACS Cycle-77 Stage 8: auditor cold-move
set -uo pipefail

AUDITOR="agents/evolve-auditor.md"
REFERENCE="agents/evolve-auditor-reference.md"
ADR="docs/architecture/adr/0015-auditor-cold-move-stage8.md"

# AC1: line count reduction >= 10% (333 -> <= 300)
line_count=$(wc -l < "$AUDITOR")
if [ "$line_count" -gt 300 ]; then
    echo "FAIL AC1: $AUDITOR has $line_count lines (expected <= 300)" >&2
    exit 1
fi

# AC2: reference section added
if ! grep -q '^## Section: output-template' "$REFERENCE"; then
    echo "FAIL AC2: '## Section: output-template' not found in $REFERENCE" >&2
    exit 1
fi

# AC3: pointer exists in persona (bash ERE, no SIGPIPE risk)
if ! grep -qE 'evolve-auditor-reference\.md.*output-template' "$AUDITOR"; then
    echo "FAIL AC3: pointer to output-template not found in $AUDITOR" >&2
    exit 1
fi

# AC4: ADR-0015 exists and <= 200 lines
if [ ! -f "$ADR" ]; then
    echo "FAIL AC4: $ADR not found" >&2
    exit 1
fi
adr_lines=$(wc -l < "$ADR")
if [ "$adr_lines" -gt 200 ]; then
    echo "FAIL AC4: $ADR has $adr_lines lines (expected <= 200)" >&2
    exit 1
fi

echo "PASS: AC1 (lines=$line_count <= 300), AC2 (output-template section in reference), AC3 (pointer in persona), AC4 (ADR-0015 exists, $adr_lines lines)"
