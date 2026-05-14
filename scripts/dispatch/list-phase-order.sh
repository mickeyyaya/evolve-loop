#!/usr/bin/env bash
#
# list-phase-order.sh — Emit phase names in order from phase-registry.json.
#
# v1 (cycle 55): data-only helper. Reads docs/architecture/phase-registry.json
# and emits one phase name per line. Falls back to hardcoded order when:
#   - EVOLVE_USE_PHASE_REGISTRY=0, or
#   - registry file absent or unparseable.
#
# This helper is NOT wired into run-cycle.sh or orchestrator.md in cycle 55.
# It serves as the reading layer for cycle 56's registry-driven dispatch.
#
# Usage:
#   bash scripts/dispatch/list-phase-order.sh
#
# Env vars:
#   EVOLVE_USE_PHASE_REGISTRY  Default "1". Set "0" to always use hardcoded order.
#   EVOLVE_PROJECT_ROOT        Repo root (falls back to git rev-parse).
#
# Output:
#   One phase name per line on stdout.
#
# Exit codes:
#   0  — success (registry or hardcoded)
#   1  — registry parse error (and EVOLVE_USE_PHASE_REGISTRY=1, file present)

set -uo pipefail

REPO_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
REGISTRY_PATH="$REPO_ROOT/docs/architecture/phase-registry.json"

emit_hardcoded_order() {
    printf '%s\n' \
        intent \
        scout \
        triage \
        plan-review \
        tdd \
        build \
        tester \
        audit \
        ship \
        retrospective \
        memo
}

if [ "${EVOLVE_USE_PHASE_REGISTRY:-1}" = "0" ] || [ ! -f "$REGISTRY_PATH" ]; then
    emit_hardcoded_order
    exit 0
fi

phase_names=$(jq -r '.phases[].name' "$REGISTRY_PATH" 2>/dev/null)
if [ $? -ne 0 ] || [ -z "$phase_names" ]; then
    echo "[list-phase-order] ERROR: failed to parse $REGISTRY_PATH" >&2
    exit 1
fi

echo "$phase_names"
exit 0
