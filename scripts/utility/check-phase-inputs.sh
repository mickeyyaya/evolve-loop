#!/usr/bin/env bash
#
# check-phase-inputs.sh — Verify all declared inputs for a phase are present.
#
# Usage:
#   bash scripts/utility/check-phase-inputs.sh <phase> <cycle>
#
# Exit codes:
#   0  — all declared inputs present
#   1  — one or more inputs missing (list printed to stdout)
#   2  — phase-registry.json not found, or unknown phase, or jq missing
#
# Reads phase input declarations from docs/architecture/phase-registry.json.
# Checks:
#   - Each file in inputs.files[] (with {cycle} substituted)
#   - Each field in inputs.state_fields[] exists in .evolve/state.json
#
# bash 3.2 compatible. No declare -A, no mapfile, no GNU-only flags.

set -uo pipefail

_self="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$_self/../lifecycle/resolve-roots.sh"

PHASE="${1:?usage: check-phase-inputs.sh <phase> <cycle>}"
CYCLE="${2:?usage: check-phase-inputs.sh <phase> <cycle>}"

# jq is required
command -v jq >/dev/null 2>&1 || { echo "ERROR: jq required but not found in PATH" >&2; exit 2; }

# Locate phase-registry.json: try PROJECT_ROOT, git toplevel, then PLUGIN_ROOT.
REGISTRY=""
if [ -f "$EVOLVE_PROJECT_ROOT/docs/architecture/phase-registry.json" ]; then
    REGISTRY="$EVOLVE_PROJECT_ROOT/docs/architecture/phase-registry.json"
else
    _git_top="$(git rev-parse --show-toplevel 2>/dev/null || true)"
    if [ -n "$_git_top" ] && [ -f "$_git_top/docs/architecture/phase-registry.json" ]; then
        REGISTRY="$_git_top/docs/architecture/phase-registry.json"
    elif [ -f "$EVOLVE_PLUGIN_ROOT/docs/architecture/phase-registry.json" ]; then
        REGISTRY="$EVOLVE_PLUGIN_ROOT/docs/architecture/phase-registry.json"
    fi
fi
[ -n "$REGISTRY" ] || { echo "ERROR: phase-registry.json not found (tried PROJECT_ROOT, git-toplevel, PLUGIN_ROOT)" >&2; exit 2; }

# Look up the phase in the registry
phase_json=$(jq -r --arg p "$PHASE" '.phases[] | select(.name == $p) | .' "$REGISTRY" 2>/dev/null || true)
if [ -z "$phase_json" ] || [ "$phase_json" = "null" ]; then
    echo "ERROR: unknown phase: $PHASE (not in phase-registry.json)" >&2
    exit 2
fi

# Extract input files list (one path per line; may be empty)
input_files=$(printf '%s\n' "$phase_json" | jq -r '.inputs.files[] // empty' 2>/dev/null || true)

# Extract required state fields (one field name per line; may be empty)
state_fields=$(printf '%s\n' "$phase_json" | jq -r '.inputs.state_fields[] // empty' 2>/dev/null || true)

missing_count=0

# Check each required file
if [ -n "$input_files" ]; then
    while IFS= read -r f; do
        [ -z "$f" ] && continue
        resolved=$(printf '%s\n' "$f" | sed "s/{cycle}/$CYCLE/g")
        abs_path="$EVOLVE_PROJECT_ROOT/$resolved"
        if [ ! -f "$abs_path" ]; then
            printf 'MISSING: %s\n' "$resolved"
            missing_count=$((missing_count + 1))
        fi
    done << FILELIST
$input_files
FILELIST
fi

# Check state.json field presence
STATE="$EVOLVE_PROJECT_ROOT/.evolve/state.json"
if [ -n "$state_fields" ]; then
    if [ ! -f "$STATE" ]; then
        while IFS= read -r field; do
            [ -z "$field" ] && continue
            printf 'MISSING: state.json field: %s (state.json absent)\n' "$field"
            missing_count=$((missing_count + 1))
        done << FIELDLIST
$state_fields
FIELDLIST
    else
        while IFS= read -r field; do
            [ -z "$field" ] && continue
            if ! jq -e --arg f "$field" 'has($f)' "$STATE" >/dev/null 2>&1; then
                printf 'MISSING: state.json field: %s\n' "$field"
                missing_count=$((missing_count + 1))
            fi
        done << FIELDLIST2
$state_fields
FIELDLIST2
    fi
fi

if [ "$missing_count" -gt 0 ]; then
    exit 1
fi

printf 'OK: all inputs present for phase=%s cycle=%s\n' "$PHASE" "$CYCLE"
exit 0
