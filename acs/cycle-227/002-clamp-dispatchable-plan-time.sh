#!/usr/bin/env bash
# AC-ID:         cycle-227-002
# Description:   clampDispatchable drops undispatchable plan entries at plan-apply time; never drops floor phases
# Evidence:      go/internal/core/orchestrator.go:1230,2312 + go/internal/core/clamp_dispatchable_test.go
# Author:        tester
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: build-report.md AC#2 — clampDispatchable plan-time dispatch gate (Mode 2 fix)

set -uo pipefail

WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"

ORCH="$WORKTREE/go/internal/core/orchestrator.go"
[ -f "$ORCH" ] || { echo "RED: $ORCH not found" >&2; exit 1; }

# Structural: clampDispatchable must exist AND be wired at plan-application site.
grep -q "func (o \*Orchestrator) clampDispatchable(" "$ORCH" \
  || { echo "RED: clampDispatchable not defined in $ORCH" >&2; exit 1; }

grep -q "clampDispatchable(clampedPlan)" "$ORCH" \
  || { echo "RED: clampDispatchable not called at plan-application site in $ORCH" >&2; exit 1; }

# Behavioral: exercise all four contract cases — undispatchable phase dropped,
# no-runner phase dropped, preflight failure drops, floor never dropped.
cd "$WORKTREE/go" || { echo "RED: cannot cd to $WORKTREE/go" >&2; exit 1; }

if ! go test ./internal/core/... \
  -run "TestClampPlan_UndispatchableDroppedWithWarn|TestNoCrash_PersonalessPhaseInPlan|TestClampPlan_PreflightFailureDropsPhase|TestClampDispatchable_NeverDropsFloor" \
  -timeout 60s 2>&1; then
  echo "RED: clampDispatchable test suite FAILED" >&2
  exit 1
fi

echo "GREEN: clampDispatchable wired and tested — undispatchable dropped, floor never dropped" >&2
exit 0
