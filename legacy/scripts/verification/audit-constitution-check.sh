#!/usr/bin/env bash
#
# audit-constitution-check.sh — Layer 4 of Reward-Hacking Defense System (ADR-0012)
#
# Verifies that audit-report.md cites the 8 constitutional principles (P1-P8)
# per criterion. Forces the Auditor's reasoning to be anchored to specific
# evidence-based principles rather than vibes.
#
# Bash 3.2 compatible.
#
# Usage:
#   audit-constitution-check.sh <audit-report.md>
#
# Exit codes:
#   0 = adequate citation coverage (≥1 principle per criterion, P1 cited at least once)
#   2 = missing citations (`principle-citation-missing` defect)
#   3 = audit-report.md missing or unreadable
#
# What it checks:
#   1. The audit-report file exists and is readable
#   2. At least one of P1..P8 is cited anywhere in the file (any P-code)
#   3. P1 (Artifact citation) is cited at least once (most criteria need it)
#   4. Reports per-principle citation counts so reviewers see coverage breadth

set -uo pipefail

if [ $# -lt 1 ]; then
    echo "Usage: $0 <audit-report.md>" >&2
    exit 3
fi

AUDIT="$1"

if [ ! -f "$AUDIT" ]; then
    echo "[constitution-check] ERROR: audit-report not found: $AUDIT" >&2
    exit 3
fi

if [ ! -r "$AUDIT" ]; then
    echo "[constitution-check] ERROR: audit-report not readable: $AUDIT" >&2
    exit 3
fi

# Count citations of each principle (case-sensitive; expect P1, P2, ..., P8)
declare_count() {
    local p="$1"
    grep -oE "\b${p}\b" "$AUDIT" 2>/dev/null | wc -l | tr -d ' '
}

P1=$(declare_count P1)
P2=$(declare_count P2)
P3=$(declare_count P3)
P4=$(declare_count P4)
P5=$(declare_count P5)
P6=$(declare_count P6)
P7=$(declare_count P7)
P8=$(declare_count P8)

TOTAL=$((P1 + P2 + P3 + P4 + P5 + P6 + P7 + P8))

echo "[constitution-check] Citations in $(basename "$AUDIT"):"
echo "  P1 (Artifact citation):           $P1"
echo "  P2 (Truthable-metric enforcement): $P2"
echo "  P3 (Prefix coherence):             $P3"
echo "  P4 (Hypothesis falsifiability):    $P4"
echo "  P5 (INERT discipline):             $P5"
echo "  P6 (Confidence honesty):           $P6"
echo "  P7 (Cross-cycle attribution):      $P7"
echo "  P8 (Substance over labeling):      $P8"
echo "  TOTAL citations:                   $TOTAL"

# Rule 1: at least 1 total citation
if [ "$TOTAL" -lt 1 ]; then
    echo "" >&2
    echo "[constitution-check] FAIL: zero principle citations in audit-report" >&2
    echo "[constitution-check] Audit verdicts must cite at least one of P1..P8 per criterion." >&2
    echo "[constitution-check] See docs/architecture/audit-constitution.md for the principle definitions." >&2
    exit 2
fi

# Rule 2: P1 must be cited at least once
if [ "$P1" -lt 1 ]; then
    echo "" >&2
    echo "[constitution-check] FAIL: P1 (Artifact citation) not cited" >&2
    echo "[constitution-check] Every audit MUST cite at least one artifact (jq query, file path, git output)." >&2
    echo "[constitution-check] Audit-report content without P1 is vibe-based reasoning, not evidence-based." >&2
    exit 2
fi

echo ""
echo "[constitution-check] PASS: adequate citation coverage (TOTAL=$TOTAL, P1 cited $P1 time(s))"
exit 0
