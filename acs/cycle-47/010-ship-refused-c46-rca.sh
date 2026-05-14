#!/usr/bin/env bash
# ACS predicate: 010 — cycle-46 ship-refused RCA doc exists with required sections
# cycle: 47
# task: T2 (ship-refused-c46 investigation)
# severity: HIGH
set -uo pipefail

RCA_DOC="docs/incidents/cycle-46-ship-refused.md"

if [ ! -f "$RCA_DOC" ]; then
    echo "FAIL: RCA document $RCA_DOC does not exist" >&2
    exit 1
fi

if ! grep -q '## Root Cause' "$RCA_DOC" 2>/dev/null; then
    echo "FAIL: $RCA_DOC missing '## Root Cause' section" >&2
    exit 1
fi

if ! grep -q '## Fix' "$RCA_DOC" 2>/dev/null; then
    echo "FAIL: $RCA_DOC missing '## Fix' section" >&2
    exit 1
fi

echo "PASS: $RCA_DOC exists with Root Cause and Fix sections"
exit 0
