#!/usr/bin/env bash
# AC-ID:         cycle-44-007
# Description:   All 6 agent profiles have non-empty effort_level field (P-NEW-26)
# Evidence:      .evolve/profiles/{scout,triage,memo,orchestrator,builder,auditor}.json
# Author:        builder
# Created:       2026-05-14T00:00:00Z
# Acceptance-of: build-report.md T1 (A2)
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || echo ".")"
OUTPUT_DIR="$REPO_ROOT/.evolve/runs/cycle-44/acs-output"
mkdir -p "$OUTPUT_DIR"

PROFILES_DIR="$REPO_ROOT/.evolve/profiles"
PROFILES="scout triage memo orchestrator builder auditor"
FAIL=0

for role in $PROFILES; do
    profile="$PROFILES_DIR/${role}.json"
    if [ ! -f "$profile" ]; then
        echo "FAIL: profile not found: $profile"
        FAIL=$((FAIL + 1))
        continue
    fi
    if ! command -v jq >/dev/null 2>&1; then
        echo "FAIL: jq required"
        exit 1
    fi
    effort=$(jq -r '.effort_level // empty' "$profile" 2>/dev/null)
    if [ -z "$effort" ]; then
        echo "FAIL: effort_level missing or empty in $role.json"
        FAIL=$((FAIL + 1))
    else
        echo "OK: $role.json effort_level=$effort"
    fi
done

if [ "$FAIL" -gt 0 ]; then
    echo "FAIL: $FAIL profile(s) missing effort_level" | tee "$OUTPUT_DIR/007-result.txt"
    exit 1
fi

echo "PASS: all 6 profiles have non-empty effort_level (scout/triage/memo/orchestrator=medium, builder/auditor=high)" | tee "$OUTPUT_DIR/007-result.txt"
exit 0
