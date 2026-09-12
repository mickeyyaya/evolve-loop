package core

import (
	"context"
	"testing"
)

// shipped_via_build_own_ship_test.go — a cycle that shipped nothing must not be
// labelled SHIPPED_VIA_BUILD because a SIBLING lane moved main.
//
// Live incident (two-wave health batch, 2026-09-12): cycle 1630 ran scout and
// triage, triage honestly committed `top_n: []` (its inbox claim had failed),
// and the host ended the cycle `triage-empty-commitment`. Between cycle start
// and closeout a sibling lane landed on main, so the pre/post HEAD probes
// differed — and finalizeOutcome relabelled the SKIPPED no-work verdict as
// SHIPPED_VIA_BUILD (loop-20260912-healthcheck.log:197). Three consumers then
// misread the cycle: IsTriageNoWorkResult no longer held (the shortened ledger
// floor was refused), the throughput recorder credited a cycle that committed
// nothing, and the dossier recorded PASS. No persona instructs an inline ship
// and lane commits carry no per-cycle trailer, so HEAD movement can never prove
// THIS cycle shipped; the only evidence is the cycle's own ship latch
// (CycleState.Shipped, set by latchShippedState on both dispatch roots).
//
// The fixture is cycle 1630's exact shape: the composed RunCycle path (not a
// hand-built result), a real triage-decision.json with an empty commitment, and
// a HEAD probe that answers differently at cycle start and at closeout.

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
	// probe at closeout return different SHAs, exactly as cycle 1630 saw.
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

// The closeout is the one consumer of the latch: completeCycle hands the
// persisted CycleState (with its Shipped flag) to finalizeCycle. No composed path in the default
// pipeline yields a SKIPPED final verdict AFTER a ship PASS (the floor guard
// declines post-ship non-floor verdicts, and an abort returns before closeout),
// so a closeout that silently dropped the latch would leave every RunCycle test
// green. This pins the seam directly, in the one shape that can reach it: a
// configured ship-floor override whose post-ship floor phase SKIPPED. The
// negative row is the cycle-1630 twin at the same seam.
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

// The latch has ONE home — the persisted CycleState — written by both dispatch
// roots at the moment the ship phase PASSes and survives the deliverable
// review. This pins the fresh root's writer site; the resume root's twin is
// TestResumeLifecycle_ShipPassPersistsTheShipLatch (integration-tagged). Drop
// either and that root reports a shipped cycle as SKIPPED_UNKNOWN after a
// pause/resume.
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
