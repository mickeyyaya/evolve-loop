#!/usr/bin/env bash
#
# init-standalone-cycle.sh — Bootstrap a cycle for single-phase execution.
#
# Usage:
#   bash scripts/utility/init-standalone-cycle.sh \
#     --cycle N \
#     --phase <registry-phase-name> \
#     [--workspace PATH] \
#     [--force-overwrite]
#
# Creates:
#   $EVOLVE_PROJECT_ROOT/.evolve/cycle-state.json  — runtime phase state
#   $EVOLVE_PROJECT_ROOT/.evolve/state.json         — minimal project state (if absent)
#   $workspace/                                      — workspace directory
#
# For --phase build: also provisions a git worktree at
#   $EVOLVE_PROJECT_ROOT/.evolve/worktrees/cycle-$CYCLE
#
# Exit codes:
#   0  — initialized successfully
#   1  — initialization failed (clobber guard, registry missing, etc.)
#
# bash 3.2 compatible. No declare -A, no mapfile, no GNU-only flags.

set -uo pipefail

_self="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$_self/../lifecycle/resolve-roots.sh"

# ── Arg parsing ──────────────────────────────────────────────────────────────

CYCLE=""
PHASE_ARG=""
WORKSPACE_ARG=""
FORCE_OVERWRITE=0

while [ $# -gt 0 ]; do
    case "$1" in
        --cycle)
            shift
            CYCLE="${1:?--cycle requires a value}"
            ;;
        --phase)
            shift
            PHASE_ARG="${1:?--phase requires a value}"
            ;;
        --workspace)
            shift
            WORKSPACE_ARG="${1:?--workspace requires a value}"
            ;;
        --force-overwrite)
            FORCE_OVERWRITE=1
            ;;
        --*)
            echo "ERROR: unknown flag: $1" >&2
            exit 1
            ;;
        *)
            echo "ERROR: unexpected positional argument: $1" >&2
            exit 1
            ;;
    esac
    shift
done

[ -n "$CYCLE" ]    || { echo "ERROR: --cycle is required" >&2; exit 1; }
[ -n "$PHASE_ARG" ] || { echo "ERROR: --phase is required" >&2; exit 1; }

# Validate CYCLE is a positive integer
case "$CYCLE" in
    ''|*[!0-9]*) echo "ERROR: --cycle must be a positive integer, got: $CYCLE" >&2; exit 1 ;;
esac
[ "$CYCLE" -gt 0 ] 2>/dev/null || { echo "ERROR: --cycle must be > 0, got: $CYCLE" >&2; exit 1; }

# ── Dependency check ─────────────────────────────────────────────────────────

command -v jq >/dev/null 2>&1 || { echo "ERROR: jq required but not found in PATH" >&2; exit 1; }

# ── Locate phase-registry.json ───────────────────────────────────────────────

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
[ -n "$REGISTRY" ] || { echo "ERROR: phase-registry.json not found (tried PROJECT_ROOT, git-toplevel, PLUGIN_ROOT)" >&2; exit 1; }

# ── Look up phase in registry ─────────────────────────────────────────────────

phase_json=$(jq -r --arg p "$PHASE_ARG" '.phases[] | select(.name == $p) | .' "$REGISTRY" 2>/dev/null || true)
if [ -z "$phase_json" ] || [ "$phase_json" = "null" ]; then
    echo "ERROR: unknown phase: $PHASE_ARG (not found in phase-registry.json)" >&2
    exit 1
fi

# Get the role from the registry (= active_agent)
ROLE=$(printf '%s\n' "$phase_json" | jq -r '.role // empty' 2>/dev/null || true)
[ -n "$ROLE" ] || { echo "ERROR: phase $PHASE_ARG has no role in phase-registry.json" >&2; exit 1; }

# ── Map registry phase name to cycle-state phase value ───────────────────────
# cycle-state.sh uses different internal names than phase-registry.
# Differences: scout→research, tester→test. All others map 1:1.

MAPPED_PHASE=""
case "$PHASE_ARG" in
    intent)       MAPPED_PHASE="intent" ;;
    scout)        MAPPED_PHASE="research" ;;
    triage)       MAPPED_PHASE="triage" ;;
    plan-review)  MAPPED_PHASE="plan-review" ;;
    tdd)          MAPPED_PHASE="tdd" ;;
    build)        MAPPED_PHASE="build" ;;
    tester)       MAPPED_PHASE="test" ;;
    audit)        MAPPED_PHASE="audit" ;;
    ship)         MAPPED_PHASE="ship" ;;
    retrospective) MAPPED_PHASE="retrospective" ;;
    memo)         MAPPED_PHASE="learn" ;;
    *)
        echo "ERROR: no cycle-state mapping for phase: $PHASE_ARG" >&2
        exit 1
        ;;
esac

# ── Clobber guard ─────────────────────────────────────────────────────────────

CYCLE_STATE_FILE="$EVOLVE_PROJECT_ROOT/.evolve/cycle-state.json"

if [ -f "$CYCLE_STATE_FILE" ] && [ "$FORCE_OVERWRITE" != "1" ]; then
    existing_phase=$(jq -r '.phase // empty' "$CYCLE_STATE_FILE" 2>/dev/null || true)
    if [ -n "$existing_phase" ] && [ "$existing_phase" != "null" ]; then
        echo "ERROR: cycle-state.json already active (phase=$existing_phase). Use --force-overwrite to clobber." >&2
        exit 1
    fi
fi

# ── Resolve workspace path ────────────────────────────────────────────────────

if [ -n "$WORKSPACE_ARG" ]; then
    WORKSPACE="$WORKSPACE_ARG"
else
    WORKSPACE="$EVOLVE_PROJECT_ROOT/.evolve/runs/cycle-$CYCLE"
fi

# Workspace path relative to EVOLVE_PROJECT_ROOT (for cycle-state.json storage)
WORKSPACE_REL=".evolve/runs/cycle-$CYCLE"

mkdir -p "$WORKSPACE"
mkdir -p "$EVOLVE_PROJECT_ROOT/.evolve"

# ── Write minimal state.json if absent ───────────────────────────────────────

STATE="$EVOLVE_PROJECT_ROOT/.evolve/state.json"
if [ ! -f "$STATE" ]; then
    state_json='{"lastCycleNumber":0,"version":"standalone","failedApproaches":[],"carryoverTodos":[],"instinctSummary":"","ledgerSummary":""}'
    printf '%s\n' "$state_json" > "${STATE}.tmp.$$"
    mv -f "${STATE}.tmp.$$" "$STATE"
fi

# ── Write cycle-state.json ────────────────────────────────────────────────────

NOW=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

cycle_state_json=$(jq -n \
    --argjson cycle_id "$CYCLE" \
    --arg phase "$MAPPED_PHASE" \
    --arg started_at "$NOW" \
    --arg phase_started_at "$NOW" \
    --arg active_agent "$ROLE" \
    --arg workspace_path "$WORKSPACE_REL" \
    '{
        cycle_id: $cycle_id,
        phase: $phase,
        started_at: $started_at,
        phase_started_at: $phase_started_at,
        active_agent: $active_agent,
        active_worktree: null,
        completed_phases: [],
        workspace_path: $workspace_path,
        intent_required: false
    }')

printf '%s\n' "$cycle_state_json" > "${CYCLE_STATE_FILE}.tmp.$$"
mv -f "${CYCLE_STATE_FILE}.tmp.$$" "$CYCLE_STATE_FILE"

# ── Worktree provisioning (build phase only) ──────────────────────────────────

if [ "$PHASE_ARG" = "build" ]; then
    WORKTREE_PATH="$EVOLVE_PROJECT_ROOT/.evolve/worktrees/cycle-$CYCLE"
    mkdir -p "$EVOLVE_PROJECT_ROOT/.evolve/worktrees"
    if [ ! -d "$WORKTREE_PATH" ]; then
        if git -C "$EVOLVE_PROJECT_ROOT" worktree add -b "evolve/cycle-$CYCLE" "$WORKTREE_PATH" HEAD 2>/dev/null; then
            # Update active_worktree in cycle-state.json
            updated=$(jq --arg wt "$WORKTREE_PATH" '.active_worktree = $wt' "$CYCLE_STATE_FILE")
            printf '%s\n' "$updated" > "${CYCLE_STATE_FILE}.tmp.$$"
            mv -f "${CYCLE_STATE_FILE}.tmp.$$" "$CYCLE_STATE_FILE"
        else
            echo "WARN: [init-standalone-cycle] git worktree add failed for build phase — active_worktree remains null" >&2
        fi
    else
        echo "WARN: [init-standalone-cycle] worktree already exists at $WORKTREE_PATH — reusing" >&2
        updated=$(jq --arg wt "$WORKTREE_PATH" '.active_worktree = $wt' "$CYCLE_STATE_FILE")
        printf '%s\n' "$updated" > "${CYCLE_STATE_FILE}.tmp.$$"
        mv -f "${CYCLE_STATE_FILE}.tmp.$$" "$CYCLE_STATE_FILE"
    fi
fi

# ── Warn about missing inputs ─────────────────────────────────────────────────
# Advisory only — init succeeds even if inputs are missing.
# Caller should run check-phase-inputs.sh to gate on prerequisites.

input_files=$(printf '%s\n' "$phase_json" | jq -r '.inputs.files[] // empty' 2>/dev/null || true)
if [ -n "$input_files" ]; then
    while IFS= read -r f; do
        [ -z "$f" ] && continue
        resolved=$(printf '%s\n' "$f" | sed "s/{cycle}/$CYCLE/g")
        abs_path="$EVOLVE_PROJECT_ROOT/$resolved"
        if [ ! -f "$abs_path" ]; then
            echo "WARN: [init-standalone-cycle] missing input: $resolved" >&2
        fi
    done << FILELIST
$input_files
FILELIST
fi

echo "[init-standalone-cycle] initialized cycle=$CYCLE phase=$PHASE_ARG (state=$MAPPED_PHASE agent=$ROLE) workspace=$WORKSPACE"
exit 0
