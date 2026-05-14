#!/usr/bin/env bash
# ACS predicate 007 — cycle 50
# reconcile-carryover-todos.sh promotes research cache on PASS verdict
#
# AC-ID: cycle-50-007
# Description: reconcile-carryover-todos.sh calls promote-research-cache.sh on PASS
# Evidence: scripts/lifecycle/reconcile-carryover-todos.sh:315-317
# Author: builder (evolve-builder)
# Created: 2026-05-14T13:55:00Z
# Acceptance-of: build-report.md AC-7
#
# metadata:
#   id: 007-reconcile-promote-on-pass
#   cycle: 50
#   task: research-cache-phase-b
#   severity: HIGH
set -uo pipefail

REPO_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
RECONCILE="$REPO_ROOT/scripts/lifecycle/reconcile-carryover-todos.sh"
[ -f "$RECONCILE" ] || { echo "RED: $RECONCILE not found"; exit 1; }

rc=0

# AC1: promote-research-cache.sh call exists in reconcile script
if ! grep -q "promote-research-cache.sh" "$RECONCILE"; then
    echo "RED AC1: 'promote-research-cache.sh' not found in reconcile-carryover-todos.sh (PASS promotion missing)"
    rc=1
else
    echo "GREEN AC1: promote-research-cache.sh call found in reconcile-carryover-todos.sh"
fi

# AC2: call passes CYCLE and WORKSPACE (verifies it's the real promotion call)
if ! grep -q 'promote-research-cache\.sh.*\$CYCLE.*\$WORKSPACE\|promote-research-cache\.sh.*"$CYCLE".*"$WORKSPACE"' "$RECONCILE"; then
    echo "RED AC2: promote-research-cache.sh call does not pass CYCLE and WORKSPACE arguments"
    rc=1
else
    echo "GREEN AC2: promote-research-cache.sh called with CYCLE and WORKSPACE arguments"
fi

exit $rc
