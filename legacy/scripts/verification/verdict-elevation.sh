#!/usr/bin/env bash
#
# verdict-elevation.sh — Layer 5 of Reward-Hacking Defense System (ADR-0012)
#
# Auto-elevates PASS verdicts with confidence < 0.85 to WARN. Forces the
# Auditor's self-reported confidence to match actual evidence strength.
# Closes the "PASS @ 0.65 confidence" loophole where cycles ship under
# fluent mode despite the Auditor's own low-confidence verdict.
#
# Bash 3.2 compatible.
#
# Usage:
#   verdict-elevation.sh <audit-report.md> [<acs-verdict.json>]
#
# Inputs:
#   audit-report.md  — Auditor's narrative report. Must contain `**Confidence:**`
#                       line with numeric value (0.0-1.0).
#   acs-verdict.json — Optional. Structured verdict file. If present, the
#                       script reads and updates .verdict and adds .elevation_reason.
#
# Behavior:
#   If verdict is PASS AND confidence < 0.85:
#     - Print "ELEVATED: PASS @ <conf> → WARN (confidence below 0.85)" to stderr
#     - If acs-verdict.json supplied: update verdict to WARN, add elevation_reason
#     - Exit 0 (success — elevation IS the success path)
#   Otherwise:
#     - Print "NO-OP: verdict=$verdict confidence=$conf" to stderr
#     - Exit 0 (no change needed)
#
# Exit codes:
#   0 = success (elevated or no-op)
#   2 = parse error (could not find confidence in audit-report)
#   3 = bad arguments

set -uo pipefail

THRESHOLD="${EVOLVE_PASS_CONFIDENCE_THRESHOLD:-0.85}"

if [ $# -lt 1 ]; then
    echo "Usage: $0 <audit-report.md> [<acs-verdict.json>]" >&2
    exit 3
fi

AUDIT="$1"
VERDICT_JSON="${2:-}"

if [ ! -f "$AUDIT" ]; then
    echo "[verdict-elevation] ERROR: audit-report not found: $AUDIT" >&2
    exit 3
fi

# Extract verdict and confidence from audit-report
# Look for **PASS**, **WARN**, **FAIL** marker
VERDICT=""
if grep -qE '\*\*(PASS|WARN|FAIL)\*\*' "$AUDIT"; then
    VERDICT=$(grep -oE '\*\*(PASS|WARN|FAIL)\*\*' "$AUDIT" | head -1 | tr -d '*')
fi

# Look for confidence number — handle "**Confidence:** 0.95" or "Confidence: 0.95"
CONFIDENCE=""
CONFIDENCE=$(grep -oE 'Confidence:?\*?\*? *[0-9]+\.[0-9]+' "$AUDIT" | head -1 | grep -oE '[0-9]+\.[0-9]+')

if [ -z "$VERDICT" ]; then
    echo "[verdict-elevation] WARN: no verdict marker found in $AUDIT — no-op" >&2
    exit 0
fi

if [ -z "$CONFIDENCE" ]; then
    echo "[verdict-elevation] WARN: no Confidence: line found in $AUDIT — no-op" >&2
    exit 0
fi

# Compare CONFIDENCE < THRESHOLD using bc (or awk fallback)
BELOW_THRESHOLD=0
if command -v bc >/dev/null 2>&1; then
    if [ "$(echo "$CONFIDENCE < $THRESHOLD" | bc 2>/dev/null)" = "1" ]; then
        BELOW_THRESHOLD=1
    fi
else
    # awk fallback
    BELOW_THRESHOLD=$(awk -v c="$CONFIDENCE" -v t="$THRESHOLD" 'BEGIN { print (c < t) ? 1 : 0 }')
fi

if [ "$VERDICT" = "PASS" ] && [ "$BELOW_THRESHOLD" = "1" ]; then
    echo "[verdict-elevation] ELEVATED: PASS @ $CONFIDENCE → WARN (confidence below $THRESHOLD threshold)" >&2

    # If acs-verdict.json provided, update it
    if [ -n "$VERDICT_JSON" ] && [ -f "$VERDICT_JSON" ]; then
        if command -v jq >/dev/null 2>&1; then
            tmp="${VERDICT_JSON}.tmp.$$"
            jq --arg conf "$CONFIDENCE" --arg thresh "$THRESHOLD" \
                '.verdict = "WARN" | .elevation_reason = ("confidence " + $conf + " below " + $thresh + " threshold (Layer 5 ADR-0012)")' \
                "$VERDICT_JSON" > "$tmp" && mv "$tmp" "$VERDICT_JSON" \
                || { echo "[verdict-elevation] ERROR: jq update failed" >&2; rm -f "$tmp"; exit 2; }
            echo "[verdict-elevation] updated $VERDICT_JSON: verdict=WARN, elevation_reason added" >&2
        else
            echo "[verdict-elevation] WARN: jq missing — cannot update $VERDICT_JSON" >&2
        fi
    fi
    exit 0
fi

echo "[verdict-elevation] NO-OP: verdict=$VERDICT confidence=$CONFIDENCE threshold=$THRESHOLD" >&2
exit 0
