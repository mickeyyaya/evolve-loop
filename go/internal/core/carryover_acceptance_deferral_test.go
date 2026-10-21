package core

import (
	"testing"
	"time"
)

func TestCloseoutCarryover_OnlyAShippedCycleCarriesADeferredCriterion(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	writeWS(t, ws, "triage-report.md", "## top_n\n- prune-guard: guard prune\n\n## deferred_acceptance\n- prune-guard: acceptance 2 — defer_reason=needs the continuation registry\n")
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	now := time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)
	failed, shipped := &State{}, &State{}

	o.closeoutCarryover(failed, ws, 1773, false, now)
	o.closeoutCarryover(shipped, ws, 1773, true, now)

	if len(failed.CarryoverTodos) != 0 || len(shipped.CarryoverTodos) != 1 || shipped.CarryoverTodos[0].ID != "prune-guard-acceptance-2" {
		t.Errorf("failed = %+v, shipped = %+v; a FAIL keeps the parent item with all its criteria, so only a ship carries the deferred one", failed.CarryoverTodos, shipped.CarryoverTodos)
	}
}
