#!/usr/bin/env bash
# AC3: evolve-retrospective.md has Step 1.5 reading abnormal-events.jsonl with correct schema
# metadata: cycle=46 task=T1-phase-b ac=AC3 risk=low

set -uo pipefail

PROJECT_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
RETRO="$PROJECT_ROOT/agents/evolve-retrospective.md"

# Verify Step 1.5 exists
if ! grep -q "1\.5" "$RETRO"; then
    echo "FAIL: evolve-retrospective.md missing Step 1.5" >&2
    exit 1
fi

# Verify it references abnormal-events.jsonl
if ! grep -q "abnormal-events.jsonl" "$RETRO"; then
    echo "FAIL: evolve-retrospective.md Step 1.5 does not reference abnormal-events.jsonl" >&2
    exit 1
fi

# Verify the schema fields are present
if ! grep -q "event_type" "$RETRO"; then
    echo "FAIL: evolve-retrospective.md Step 1.5 missing event_type schema field" >&2
    exit 1
fi

if ! grep -q "remediation_hint" "$RETRO"; then
    echo "FAIL: evolve-retrospective.md Step 1.5 missing remediation_hint schema field" >&2
    exit 1
fi

echo "PASS: AC3 — evolve-retrospective.md Step 1.5 reads abnormal-events.jsonl with correct schema"
exit 0
