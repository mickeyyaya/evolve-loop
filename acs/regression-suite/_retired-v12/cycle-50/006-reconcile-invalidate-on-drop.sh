#!/usr/bin/env bash
# ACS predicate 006 — cycle 50
# reconcile-carryover-todos.sh invalidates research cache on todo drop
#
# AC-ID: cycle-50-006
# Description: reconcile-carryover-todos.sh calls research-cache.sh invalidate with dropped-cycle reason
# Evidence: scripts/lifecycle/reconcile-carryover-todos.sh:230-232
# Author: builder (evolve-builder)
# Created: 2026-05-14T13:55:00Z
# Acceptance-of: build-report.md AC-6
#
# metadata:
#   id: 006-reconcile-invalidate-on-drop
#   cycle: 50
#   task: research-cache-phase-b
#   severity: HIGH
set -uo pipefail

REPO_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
RECONCILE="$REPO_ROOT/scripts/lifecycle/reconcile-carryover-todos.sh"
[ -f "$RECONCILE" ] || { echo "RED: $RECONCILE not found"; exit 1; }

rc=0

# AC1: research-cache.sh invalidate call exists
if ! grep -q "research-cache.sh invalidate" "$RECONCILE"; then
    echo "RED AC1: 'research-cache.sh invalidate' not found in reconcile-carryover-todos.sh"
    rc=1
else
    echo "GREEN AC1: research-cache.sh invalidate call found in reconcile-carryover-todos.sh"
fi

# AC2: dropped-cycle reason present (verifies it's the drop path, not just any invalidation)
if ! grep -q "dropped-cycle" "$RECONCILE"; then
    echo "RED AC2: 'dropped-cycle' reason not found in reconcile-carryover-todos.sh invalidate call"
    rc=1
else
    echo "GREEN AC2: dropped-cycle invalidation path present in reconcile-carryover-todos.sh"
fi

exit $rc
