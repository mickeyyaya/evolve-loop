package core

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
)

func keptRungRun(st *LadderState) *cycleRun {
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil), WithWorkflowConfig(ladderWorkflow(policy.RetryLadderEnforce)))
	return &cycleRun{o: o, cs: CycleState{CycleID: 9, Ladder: st}}
}

func TestLadderForDispatch_AKeptRungCarriesTheDefectItWasRaisedFor(t *testing.T) {
	t.Parallel()
	st := &LadderState{Rung: string(retryladder.R1), Tier: "balanced", Effort: "high", Fingerprints: []string{"codes:first", "codes:floor"}}
	got := keptRungRun(st).ladderForDispatch(PhaseBuild, PhaseRequest{})
	if got.Ladder == nil || got.Ladder.Rung != retryladder.R1 || got.Ladder.Fingerprint != "codes:floor" {
		t.Fatalf("kept try = %+v, want rung R1 carrying the latest fingerprint codes:floor", got.Ladder)
	}
}

func TestLadderForDispatch_AKeptRungWithNoFingerprintNamesNoDefect(t *testing.T) {
	t.Parallel()
	st := &LadderState{Rung: string(retryladder.R2), Tier: "deep", Effort: "high"}
	got := keptRungRun(st).ladderForDispatch(PhaseBuild, PhaseRequest{})
	if got.Ladder == nil || got.Ladder.Rung != retryladder.R2 || got.Ladder.Fingerprint != "" {
		t.Fatalf("kept try = %+v, want rung R2 and no fingerprint", got.Ladder)
	}
}

func TestLadderForDispatch_R0NeverCarriesADefect(t *testing.T) {
	t.Parallel()
	st := &LadderState{Rung: string(retryladder.R0), Fingerprints: []string{"codes:floor"}}
	got := keptRungRun(st).ladderForDispatch(PhaseBuild, PhaseRequest{})
	if got.Ladder != nil {
		t.Fatalf("an R0 dispatch carries try %+v, want none", got.Ladder)
	}
}
