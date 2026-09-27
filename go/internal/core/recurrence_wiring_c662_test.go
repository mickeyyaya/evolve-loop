package core_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/recurrence"
)

func TestC662_RetroCloseoutRecordsClosureInLedger(t *testing.T) {
	root := t.TempDir()
	seedCycleStateFile(t, root)

	orch, _, _ := newTestOrchestrator(t, newRunners(map[core.Phase]core.PhaseRunner{
		core.PhaseTriage: &alwaysErrRunner{name: "triage"},
		core.PhaseRetro:  &alwaysErrRunner{name: "retro"},
	}))
	if _, err := orch.RunCycle(context.Background(), core.CycleRequest{
		ProjectRoot: root,
		GoalHash:    "test-goal",
		Context:     map[string]string{"commit_message": "test commit"},
	}); err == nil {
		t.Fatal("triage hard failure must surface as a cycle error")
	}

	led, err := recurrence.Load(filepath.Join(root, ".evolve", "recurrence-ledger.json"))
	if err != nil {
		t.Fatalf("load recurrence ledger: %v", err)
	}
	if len(led.Entries) == 0 {
		t.Fatalf("RED: recurrence-ledger.json has no entries after a FAIL cycle — " +
			"RecordClosure is not wired into the retro-closeout seam (Count()==0 forever)")
	}
	// The closure must be keyed by the failing cycle (seeded cycle_id=1).
	foundCycle := false
	for _, e := range led.Entries {
		for _, c := range e.Cycles {
			if c == 1 {
				foundCycle = true
			}
		}
	}
	if !foundCycle {
		t.Errorf("RED: no ledger entry records cycle 1 — the closure must carry the FAIL cycle number; entries=%+v", led.Entries)
	}
}
