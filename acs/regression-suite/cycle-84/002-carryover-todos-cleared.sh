#!/usr/bin/env bash
# AC-ID: cycle-84-002
# Verify state.json:carryoverTodos is schema-valid.
#
# Re-baselined 2026-06-05 (was: "carryoverTodos must be an empty array").
# The empty-array assertion contradicted the documented operator workflow —
# queueing deferred work via carryoverTodos[] is the sanctioned mechanism
# (scout reads carryoverTodos as pointers; operators queue deferred work
# there by convention). The preserved intent of cycle-84's check is that a
# cycle never leaves the field malformed: it must be an array, and every
# entry must carry the CarryoverTodo schema fields
# (go/internal/core/ports.go: id, action, priority).
set -uo pipefail
REPO_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
STATE="$REPO_ROOT/.evolve/state.json"
[ -f "$STATE" ] || { echo "RED cycle-84-002: $STATE missing"; exit 1; }

is_array=$(jq '.carryoverTodos == null or (.carryoverTodos | type == "array")' "$STATE" 2>/dev/null)
if [ "$is_array" != "true" ]; then
    echo "RED cycle-84-002: carryoverTodos is present but not an array"
    exit 1
fi

bad=$(jq '[(.carryoverTodos // []) | .[] | select((.id // "") == "" or (.action // "") == "" or (.priority // "") == "")] | length' "$STATE" 2>/dev/null)
if [ "${bad:-1}" != "0" ]; then
    echo "RED cycle-84-002: $bad carryoverTodos entries missing required fields (id/action/priority)"
    exit 1
fi

count=$(jq '(.carryoverTodos // []) | length' "$STATE" 2>/dev/null)
echo "GREEN cycle-84-002: carryoverTodos schema-valid (${count:-0} queued item(s))"
exit 0
