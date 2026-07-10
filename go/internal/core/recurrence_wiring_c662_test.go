package core_test

// recurrence_wiring_c662_test.go — cycle-662 RED contract for
// chronicle-s1-recurrence-index gap G1 (PRODUCTION WIRING). Cycle 661 landed
// recurrence.Ledger.RecordClosure with ZERO call sites, so nothing ever writes
// .evolve/recurrence-ledger.json and Count()==0 forever (the same
// consumed-without-landing disease as token-resolver-production-wiring).
//
// This test drives the EXPORTED orchestrator entry point (RunCycle) end-to-end
// through the real retro-closeout / failure-learning seam
// (writeDeterministicLearning → faillearn.WriteArtifacts), which is where the
// closure must also be recorded into the ledger. It exercises the SUT — a magic
// string cannot make RunCycle write a ledger file keyed by the failing cycle.
//
// Builder contract: wire RecordClosure at the deterministic retro-closeout seam
// (nil Escalator/Autofiler there — escalation apply stays boundary-only to avoid
// racing inboxmover.Claim), persisting to
// <root>/.evolve/recurrence-ledger.json via Load/Save under flock.
//
// RED today: RunCycle writes the lesson YAML but no recurrence-ledger.json, so
// recurrence.Load returns an empty ledger. GREEN once the closure is wired.

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/recurrence"
)

// TestC662_RetroCloseoutRecordsClosureInLedger — AC1. A FAIL cycle's lesson
// pattern must appear in .evolve/recurrence-ledger.json keyed by the cycle
// number. Uses the shared failing-triage harness (seeds cycle_id=1), which
// exercises the deterministic failure-learning path end-to-end.
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
