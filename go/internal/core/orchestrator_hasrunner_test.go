package core

import "testing"

func TestOrchestrator_HasRunner(t *testing.T) {
	t.Parallel()

	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))

	if !o.HasRunner(PhaseTriage) {
		t.Error("HasRunner(PhaseTriage) = false for a registered runner")
	}
	if o.HasRunner(Phase("no-such-phase")) {
		t.Error("HasRunner reported a runner for an unregistered phase")
	}
}
