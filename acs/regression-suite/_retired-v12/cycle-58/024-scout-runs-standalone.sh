#!/usr/bin/env bash
# ACS predicate 024 — cycle 58
# Verifies that init-standalone-cycle.sh correctly bootstraps a scout phase:
# produces cycle-state.json with phase=research, active_agent=scout, correct
# cycle_id. Also verifies subagent-run.sh --validate-profile scout passes.
# Anti-tautology: uninitialized dir does not have phase=research.
#
# AC-ID: cycle-58-024
# Description: init-standalone-cycle.sh bootstraps scout phase for standalone execution
# Evidence: scripts/utility/init-standalone-cycle.sh, scripts/dispatch/subagent-run.sh
# Author: builder (evolve-builder)
# Created: 2026-05-15T00:00:00Z
# Acceptance-of: build-report.md AC-58-2 (init-standalone-cycle utility)
#
# metadata:
#   id: 024-scout-runs-standalone
#   cycle: 58
#   task: adr5-standalone-phase-runners
#   severity: HIGH

set -uo pipefail

REPO_ROOT="${WORKTREE_PATH:-${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
INIT_SCRIPT="$REPO_ROOT/scripts/utility/init-standalone-cycle.sh"
SUBAGENT_RUN="$REPO_ROOT/scripts/dispatch/subagent-run.sh"

for _f in "$INIT_SCRIPT" "$SUBAGENT_RUN"; do
    if [ ! -f "$_f" ]; then
        echo "RED: required file not found: $_f"
        exit 1
    fi
done

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

FIXTURE_CYCLE=9999
rc=0

# ── AC1: init-standalone-cycle.sh exits 0 for scout phase ────────────────────
set +e
init_out=$(EVOLVE_PROJECT_ROOT="$TMP" bash "$INIT_SCRIPT" \
    --cycle "$FIXTURE_CYCLE" --phase scout --force-overwrite 2>&1)
init_rc=$?
set -e

if [ "$init_rc" -eq 0 ]; then
    echo "GREEN AC1: init-standalone-cycle.sh exits 0 for scout"
else
    echo "RED AC1: init-standalone-cycle.sh failed (rc=$init_rc): $init_out"
    rc=1
fi

# ── AC2: cycle-state.json has phase=research ──────────────────────────────────
CYCLE_STATE="$TMP/.evolve/cycle-state.json"
if [ ! -f "$CYCLE_STATE" ]; then
    echo "RED AC2: cycle-state.json not created at $CYCLE_STATE"
    rc=1
else
    got_phase=$(jq -r '.phase // empty' "$CYCLE_STATE" 2>/dev/null || true)
    if [ "$got_phase" = "research" ]; then
        echo "GREEN AC2: cycle-state.json has phase=research"
    else
        echo "RED AC2: expected phase=research, got phase=$got_phase"
        rc=1
    fi
fi

# ── AC3: active_agent=scout ───────────────────────────────────────────────────
if [ -f "$CYCLE_STATE" ]; then
    got_agent=$(jq -r '.active_agent // empty' "$CYCLE_STATE" 2>/dev/null || true)
    if [ "$got_agent" = "scout" ]; then
        echo "GREEN AC3: active_agent=scout"
    else
        echo "RED AC3: expected active_agent=scout, got $got_agent"
        rc=1
    fi
fi

# ── AC4: cycle_id matches fixture ─────────────────────────────────────────────
if [ -f "$CYCLE_STATE" ]; then
    got_cycle=$(jq -r '.cycle_id // empty' "$CYCLE_STATE" 2>/dev/null || true)
    if [ "$got_cycle" = "$FIXTURE_CYCLE" ]; then
        echo "GREEN AC4: cycle_id=$FIXTURE_CYCLE"
    else
        echo "RED AC4: expected cycle_id=$FIXTURE_CYCLE, got $got_cycle"
        rc=1
    fi
fi

# ── AC5: subagent-run.sh --validate-profile scout exits 0 ────────────────────
# Uses VALIDATE_ONLY mode — no LLM call, just profile + adapter load check.
set +e
EVOLVE_PROJECT_ROOT="$TMP" bash "$SUBAGENT_RUN" --validate-profile scout > /dev/null 2>&1
validate_rc=$?
set -e

if [ "$validate_rc" -eq 0 ]; then
    echo "GREEN AC5: subagent-run.sh --validate-profile scout passes"
else
    echo "RED AC5: --validate-profile scout failed (rc=$validate_rc)"
    rc=1
fi

# ── AC6 (anti-tautology): uninitialized dir does not have phase=research ─────
TMP2=$(mktemp -d)
trap 'rm -rf "$TMP2"' EXIT
phase_without_init=$(jq -r '.phase // "none"' "$TMP2/.evolve/cycle-state.json" 2>/dev/null || echo "none")
if [ "$phase_without_init" = "research" ]; then
    echo "RED AC6 (anti-tautology): uninitialized dir has phase=research (impossible)"
    rc=1
else
    echo "GREEN AC6 (anti-tautology): uninitialized dir does not have phase=research (got: $phase_without_init)"
fi

exit "$rc"
