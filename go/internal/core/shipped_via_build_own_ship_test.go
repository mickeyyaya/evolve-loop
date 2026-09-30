package core

import (
	"context"
	"testing"
)

func TestRunCycle_EmptyTriageCommitmentSurvivesSiblingLanding(t *testing.T) {
	t.Parallel()
	storage := &fakeStorage{}
	runners := buildRunners(nil)
	runners[PhaseTriage] = triageDecisionRunner{
		verdict:  VerdictPASS,
		decision: `{"top_n":[],"deferred":[{"id":"self-consistency-on-decision-phases","reason":"inbox claim failed"}]}`,
	}
	throughputCredits := 0
	o := NewOrchestrator(storage, &fakeLedger{}, runners,
		WithWorktreeProvisioner(&fakeWorktree{path: t.TempDir()}),
		WithThroughputRecorder(func(*State, int, string) { throughputCredits++ }))
	// A sibling lane lands on main mid-cycle: the probe at cycle start and the
	// probe at closeout return different SHAs.
	heads := []string{"a2e60952-cycle-start", "b496a8dc-sibling-landed"}
	o.gitHEAD = func() (string, error) {
		head := heads[0]
		if len(heads) > 1 {
			heads = heads[1:]
		}
		return head, nil
	}

	result, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "sibling-landed"})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if result.FinalVerdict != CycleOutcomeSkippedUnknown {
		t.Errorf("FinalVerdict = %q, want %q — a sibling's landing is not this cycle's ship", result.FinalVerdict, CycleOutcomeSkippedUnknown)
	}
	if !IsTriageNoWorkResult(result) {
		t.Errorf("IsTriageNoWorkResult = false for %+v — the planned no-work disposition was lost", result)
	}
	if throughputCredits != 0 {
		t.Errorf("throughput recorder credited a cycle that committed nothing (%d call(s))", throughputCredits)
	}
	if storage.cycleState.Shipped {
		t.Errorf("a cycle that never reached ship persisted Shipped=true")
	}
}

func TestCompleteCycle_ForwardsTheShipLatchToTheOutcomeLabel(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		shipped bool
		want    string
	}{
		{"own-ship-landed", true, CycleOutcomeShippedViaBuild},
		{"no-ship-this-cycle", false, CycleOutcomeSkippedUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			o := &Orchestrator{storage: &fakeUpdaterStorage{}, gitHEAD: func() (string, error) { return "same-head", nil }}
			cr := &cycleRun{
				ctx:    context.Background(),
				o:      o,
				cycle:  1,
				req:    CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "shipped-then-skipped"},
				cs:     CycleState{WorkspacePath: t.TempDir(), Shipped: tc.shipped},
				result: CycleResult{Cycle: 1, FinalVerdict: VerdictSKIPPED, PhasesRun: []Phase{PhaseBuild, PhaseAudit, PhaseShip}},
			}
			if err := cr.completeCycle(); err != nil {
				t.Fatalf("completeCycle: %v", err)
			}
			if cr.result.FinalVerdict != tc.want {
				t.Errorf("FinalVerdict = %q, want %q (shipped=%t)", cr.result.FinalVerdict, tc.want, tc.shipped)
			}
		})
	}
}

func TestRunCycle_ShipPassPersistsTheShipLatch(t *testing.T) {
	t.Parallel()
	st := &fakeStorage{}
	o := NewOrchestrator(st, &fakeLedger{}, buildRunners(nil))
	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "ships"}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if !st.cycleState.Shipped {
		t.Errorf("fresh root: ship PASS did not latch Shipped on the persisted cycle state: %+v", st.cycleState)
	}
}
