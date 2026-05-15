#!/usr/bin/env bash
# ACS predicate 058 — cycle 63
# Verifies the cycle-62 ship-refused carryover close-out task: the incident
# doc exists, has a 3-section structure (what / why / resolution), and
# references the actual abnormal-events.jsonl source.
#
# AC-ID: cycle-63-058
# Description: Close abnormal-ship-refused-c62 carryover via incident doc
# Evidence: docs/incidents/cycle-62-ship-refused.md exists with required sections
# Author: builder (cycle 63)
# Created: 2026-05-15T00:00:00Z
# Acceptance-of: scout-report.md Task 2
#
# metadata:
#   id: 058-cycle62-incident-closeout
#   cycle: 63
#   task: cycle62-ship-refused-closeout
#   severity: MEDIUM

set -uo pipefail

if [ -n "${EVOLVE_PROJECT_ROOT:-}" ]; then
    REPO_ROOT="$EVOLVE_PROJECT_ROOT"
else
    REPO_ROOT=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
fi
if [ -f "$REPO_ROOT/.git" ]; then
    REPO_ROOT="$(cd "$REPO_ROOT" && cd "$(git rev-parse --git-common-dir)/.." && pwd)"
fi

DOC="$REPO_ROOT/docs/incidents/cycle-62-ship-refused.md"
rc=0

# ── AC1: doc exists
if [ ! -f "$DOC" ]; then
    echo "RED AC1: $DOC missing"
    exit 1
fi
echo "GREEN AC1: incident doc exists"

# ── AC2: 3 required sections present
for section in "What happened" "Why this was expected" "Resolution"; do
    if ! grep -qi "^## ${section}" "$DOC"; then
        echo "RED AC2: missing section '## ${section}'"
        rc=1
    fi
done
[ "$rc" -eq 0 ] && echo "GREEN AC2: all 3 sections present (what / why / resolution)"

# ── AC3: references the actual abnormal-events stream (not just claims to)
if ! grep -q "abnormal-events.jsonl" "$DOC"; then
    echo "RED AC3: doc does not cite abnormal-events.jsonl as source"
    rc=1
else
    echo "GREEN AC3: doc cites abnormal-events.jsonl as source"
fi

# ── AC4: doc explains *why* the gate firing was expected (not just describes it)
if ! grep -qiE "(audit.bound|tree.SHA|expected_ship_sha|FAILED.AND.LEARNED)" "$DOC"; then
    echo "RED AC4: doc does not reference the audit-binding/tree-SHA mechanism that produced ship-refused events"
    rc=1
else
    echo "GREEN AC4: doc explains the audit-binding mechanism behind ship-refused"
fi

# ── AC5 (anti-tautology): doc must have substantive length, not a stub.
words=$(wc -w < "$DOC" | tr -d ' ')
if [ "${words:-0}" -lt 100 ]; then
    echo "RED AC5 (anti-tautology): doc has only ${words} words (<100, looks like a stub)"
    rc=1
else
    echo "GREEN AC5 (anti-tautology): doc has ${words} words (substantive)"
fi

exit "$rc"
