package core

import "testing"

// TestOrchestrator_HasRunner pins the composition-root read seam behind the
// cycle-563 memo-dispatch bug: a phase the router can nominate but that has
// no registered runner is silently skipped by dispatch's missing-runner
// escape hatch, so HasRunner is how tests prove a routing→dispatch handoff
// is actually wired — not just that Route() names the phase. CI's apicover
// -enforce flagged it UNCOVERED (no test named it).
func TestOrchestrator_HasRunner(t *testing.T) {
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))

	if !o.HasRunner(PhaseTriage) {
		t.Error("HasRunner(PhaseTriage) = false for a registered runner")
	}
	if o.HasRunner(Phase("no-such-phase")) {
		t.Error("HasRunner reported a runner for an unregistered phase")
	}
}
