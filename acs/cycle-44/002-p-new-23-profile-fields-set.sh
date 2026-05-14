#!/usr/bin/env bash
# AC-ID:         cycle-44-002
# Description:   All 6 required profiles have turn_budget_hint field >= 1
# Evidence:      .evolve/profiles/{scout,builder,auditor,orchestrator,memo,triage}.json
# Author:        builder
# Created:       2026-05-14T00:00:00Z
# Acceptance-of: build-report.md T1 (P-NEW-23)
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || echo ".")"
PROFILES_DIR="$REPO_ROOT/.evolve/profiles"
REQUIRED_PROFILES="scout builder auditor orchestrator memo triage"
FAIL=0

command -v jq >/dev/null 2>&1 || { echo "FAIL: jq not available"; exit 1; }

for profile in $REQUIRED_PROFILES; do
    PROFILE_FILE="$PROFILES_DIR/${profile}.json"
    [ -f "$PROFILE_FILE" ] || { echo "FAIL: profile not found: ${profile}.json"; FAIL=1; continue; }
    HINT=$(jq -r '.turn_budget_hint // empty' "$PROFILE_FILE" 2>/dev/null)
    if [ -z "$HINT" ]; then
        echo "FAIL: turn_budget_hint missing in ${profile}.json"
        FAIL=1
    elif [ "$HINT" -lt 1 ] 2>/dev/null; then
        echo "FAIL: turn_budget_hint < 1 in ${profile}.json (got $HINT)"
        FAIL=1
    else
        echo "OK: ${profile}.json turn_budget_hint=$HINT"
    fi
done

[ "$FAIL" -eq 0 ] || exit 1
echo "PASS: all 6 profiles have turn_budget_hint >= 1"
exit 0
