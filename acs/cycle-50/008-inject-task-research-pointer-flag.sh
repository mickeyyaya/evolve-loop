#!/usr/bin/env bash
# ACS predicate 008 — cycle 50
# inject-task.sh --research-pointer flag writes field to JSON output
#
# AC-ID: cycle-50-008
# Description: inject-task.sh --research-pointer flag accepted and emitted in JSON
# Evidence: scripts/utility/inject-task.sh:53,135-137
# Author: builder (evolve-builder)
# Created: 2026-05-14T13:55:00Z
# Acceptance-of: build-report.md AC-8
#
# metadata:
#   id: 008-inject-task-research-pointer-flag
#   cycle: 50
#   task: research-cache-phase-b
#   severity: MEDIUM
set -uo pipefail

REPO_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
INJECT="$REPO_ROOT/scripts/utility/inject-task.sh"
[ -f "$INJECT" ] || { echo "RED: $INJECT not found"; exit 1; }
[ -x "$INJECT" ] || { echo "RED: $INJECT not executable"; exit 1; }
command -v jq >/dev/null 2>&1 || { echo "RED: jq required"; exit 1; }

rc=0

# AC1: --research-pointer arg produces JSON with research_pointer field
json_output=$(bash "$INJECT" \
    --priority LOW \
    --action "test-predicate-action" \
    --research-pointer ".evolve/research/by-task/abc123.md" \
    --dry-run 2>/dev/null)

if [ -z "$json_output" ]; then
    echo "RED AC1: inject-task.sh --dry-run produced no output"
    rc=1
else
    rp=$(echo "$json_output" | jq -r '.research_pointer // empty' 2>/dev/null)
    if [ -z "$rp" ]; then
        echo "RED AC1: JSON output missing research_pointer field: $json_output"
        rc=1
    else
        echo "GREEN AC1: research_pointer='$rp' present in JSON output"
    fi
fi

# AC2: without --research-pointer, JSON does NOT contain research_pointer field
json_no_rp=$(bash "$INJECT" \
    --priority LOW \
    --action "test-no-research-pointer" \
    --dry-run 2>/dev/null)

if echo "$json_no_rp" | jq -e '.research_pointer' >/dev/null 2>&1; then
    echo "RED AC2: research_pointer field present even without --research-pointer flag: $json_no_rp"
    rc=1
else
    echo "GREEN AC2: research_pointer absent from JSON when --research-pointer not provided"
fi

exit $rc
