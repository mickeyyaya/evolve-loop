#!/usr/bin/env bash
# ACS predicate 004 — cycle 48
# Verifies that phase-gate.sh _check_builder_isolation_breach() calls
# _append_abnormal_event with "builder-isolation-breach" (T3 fix).
#
# metadata:
#   id: 004-isolation-breach-wires-jsonl
#   cycle: 48
#   task: T3
#   severity: MEDIUM
set -uo pipefail

PHASE_GATE="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null)}/scripts/lifecycle/phase-gate.sh"

if grep -q '_append_abnormal_event.*builder-isolation-breach' "$PHASE_GATE" 2>/dev/null; then
    echo "GREEN: phase-gate.sh wires _append_abnormal_event for builder-isolation-breach"
    exit 0
else
    echo "RED: phase-gate.sh missing _append_abnormal_event call for builder-isolation-breach"
    exit 1
fi
