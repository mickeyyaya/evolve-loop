package core

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
)

// ADR-0103 unit 07 — the ONE ship.error producer projects shiperr.SignalDebugKeys
// from the ShipError's Debug map into Event.Fields when non-empty (the triage
// whitelist: step, git_rc, worktree, branch, cycle_branch, repair_outcome) and
// never any other Debug key; a Debug without them leaves the fields as before.
func TestEmitShipError_ProjectsSignalDebugKeys(t *testing.T) {
	t.Parallel()
	c, got := recordingCenter()
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithSignalCenter(c))
	cs := CycleState{WorkspacePath: t.TempDir(), Phase: string(PhaseShip), RunID: "run-1641"}
	se := NewShipError(CodeGitPushRejected, ShipClassTransient, StageAtomicShip, "push rejected",
		shiperr.StepKey, "push", "git_rc", "1", "gate_err", "x", "branch", "main", "repair_outcome", "declined", "worktree", "")
	o.recordShipError(context.Background(), 1641, cs, se)
	if len(*got) != 1 {
		t.Fatalf("one ship.error, got %d", len(*got))
	}
	f := (*got)[0].Fields
	want := map[string]string{"class": "transient", "stage": "atomic-ship", "step": "push", "git_rc": "1", "branch": "main", "repair_outcome": "declined"}
	for k, v := range want {
		if f[k] != v {
			t.Errorf("fields[%s]=%q, want %q (all: %v)", k, f[k], v, f)
		}
	}
	if _, leaked := f["gate_err"]; leaked {
		t.Errorf("a Debug key outside the whitelist is never projected: %v", f)
	}
	if _, empty := f["worktree"]; empty {
		t.Errorf("an empty whitelisted value is not projected: %v", f)
	}
	if len(f) != len(want)+1 || f["path"] == "" {
		t.Errorf("fields = class, stage, path plus the non-empty whitelisted keys: %v", f)
	}
	*got = nil
	o.recordShipError(context.Background(), 1641, cs, NewShipError(CodeGitCommitFailed, ShipClassPrecondition, StageAtomicShip, "commit failed", "gate_err", "x"))
	if f := (*got)[0].Fields; len(f) != 3 || f["class"] == "" || f["stage"] == "" || f["path"] == "" {
		t.Errorf("a Debug without whitelisted keys leaves the fields as before: %v", f)
	}
}
