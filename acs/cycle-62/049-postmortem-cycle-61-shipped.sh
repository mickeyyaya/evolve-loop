#!/usr/bin/env bash
# ACS predicate 049 — cycle 62
# Verifies that docs/incidents/cycle-61.md was committed with required sections
# documenting the cycle 61 fallout investigation (B0-B7 root cause analysis).
#
# AC-ID: cycle-62-049
# Description: postmortem-cycle-61-shipped
# Evidence: existence + non-empty + required H2 sections + structural-fix table
# Author: builder (manual fix, Step 1 of plan)
# Created: 2026-05-15T00:00:00Z
# Acceptance-of: plan Step 1
#
# metadata:
#   id: 049-postmortem-cycle-61-shipped
#   cycle: 62
#   task: cycle-61-postmortem
#   severity: MEDIUM

set -uo pipefail

REPO_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
DOC="$REPO_ROOT/docs/incidents/cycle-61.md"

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
rc=0

# ── AC1: file exists and is non-empty ─────────────────────────────────────────
if [ -s "$DOC" ]; then
    size=$(wc -c < "$DOC" | tr -d ' ')
    echo "GREEN AC1: docs/incidents/cycle-61.md exists ($size bytes)"
else
    echo "RED AC1: docs/incidents/cycle-61.md missing or empty"
    rc=1
fi

# ── AC2: required H2 sections present ─────────────────────────────────────────
required_sections=(
    "## Summary"
    "## Timeline"
    "## Source-Verified Facts"
    "## Root Cause Analysis"
    "## Structural Fixes Required"
    "## Operator Lessons"
)
missing=0
for sec in "${required_sections[@]}"; do
    if ! grep -qF "$sec" "$DOC" 2>/dev/null; then
        echo "RED AC2: missing required section '$sec'"
        missing=$((missing + 1))
        rc=1
    fi
done
if [ "$missing" = "0" ]; then
    echo "GREEN AC2: all ${#required_sections[@]} required sections present"
fi

# ── AC3: structural-fix table references all 7 bug IDs ────────────────────────
all_bugs_referenced=1
for bug in B0 B1 B2 B3 B4 B5 B6 B7; do
    if ! grep -qE "^\| $bug \|" "$DOC" 2>/dev/null; then
        echo "RED AC3: structural-fix table missing row for $bug"
        all_bugs_referenced=0
        rc=1
    fi
done
if [ "$all_bugs_referenced" = "1" ]; then
    echo "GREEN AC3: structural-fix table covers B0-B7"
fi

# ── AC4 (anti-tautology): empty fixture must fail ─────────────────────────────
# Create an empty fixture file and verify the same checks would reject it.
EMPTY_FIXTURE="$TMP/empty-cycle-61.md"
: > "$EMPTY_FIXTURE"
fixture_passes=1
for sec in "${required_sections[@]}"; do
    if ! grep -qF "$sec" "$EMPTY_FIXTURE" 2>/dev/null; then
        fixture_passes=0
        break
    fi
done
if [ "$fixture_passes" = "0" ]; then
    echo "GREEN AC4 (anti-tautology): empty fixture correctly fails section check"
else
    echo "RED AC4 (anti-tautology): empty fixture should fail but passed — predicate is tautological"
    rc=1
fi

exit "$rc"
