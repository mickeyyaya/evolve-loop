package core

import (
	"context"
	"testing"
)

type failingShipRunner struct{ err error }

func (r *failingShipRunner) Name() string { return "ship" }
func (r *failingShipRunner) Run(_ context.Context, _ PhaseRequest) (PhaseResponse, error) {
	return PhaseResponse{Phase: "ship", Verdict: VerdictFAIL}, r.err
}

type recoveringShipRunner struct{ calls int }

func (r *recoveringShipRunner) Name() string { return "ship" }
func (r *recoveringShipRunner) Run(_ context.Context, _ PhaseRequest) (PhaseResponse, error) {
	r.calls++
	if r.calls == 1 {
		return PhaseResponse{Phase: "ship", Verdict: VerdictFAIL},
			NewShipError(CodeGitPushRejected, ShipClassTransient, StageAtomicShip, "push race")
	}
	return PhaseResponse{Phase: "ship", Verdict: VerdictPASS}, nil
}

func TestOrchestrator_ShipFailureAborts_PreservesWorktree(t *testing.T) {
	t.Parallel()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	runners[PhaseShip] = &failingShipRunner{
		err: NewShipError(CodeIntegrityTreeDrift, ShipClassIntegrity, StageAtomicShip, "tree drift"),
	}
	wt := &fakeWorktree{path: "/tmp/wt/cycle-1"}
	o := NewOrchestrator(st, led, runners, WithWorktreeProvisioner(wt))

	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g"}); err == nil {
		t.Fatal("integrity ship failure must abort the cycle")
	}
	if len(wt.cleaned) != 0 {
		t.Fatalf("worktree pruned on ship failure (cleaned=%v) — audited work must be preserved for recovery", wt.cleaned)
	}
}

func TestOrchestrator_ShipRecoversThenSucceeds_CleansWorktree(t *testing.T) {
	t.Parallel()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	ship := &recoveringShipRunner{}
	runners[PhaseShip] = ship
	wt := &fakeWorktree{path: "/tmp/wt/cycle-1"}
	o := NewOrchestrator(st, led, runners, WithWorktreeProvisioner(wt))

	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g"}); err != nil {
		t.Fatalf("transient ship failure should recover: %v", err)
	}
	if ship.calls != 2 {
		t.Fatalf("ship calls = %d, want 2 (fail → retry)", ship.calls)
	}
	if len(wt.cleaned) != 1 || wt.cleaned[0] != "/tmp/wt/cycle-1" {
		t.Fatalf("worktree must be cleaned after the ship eventually succeeds; cleaned=%v", wt.cleaned)
	}
}

func TestPreserveOnVerdict(t *testing.T) {
	cases := []struct {
		verdict string
		want    bool
	}{
		{VerdictFAIL, true},
		{VerdictPASS, false},
		{VerdictWARN, false},
		{CycleOutcomeShippedViaBuild, false},
		{CycleOutcomeSkippedUnknown, false},
		{"", false},
	}
	for _, tc := range cases {
		if got := preserveOnVerdict(tc.verdict); got != tc.want {
			t.Errorf("preserveOnVerdict(%q) = %v, want %v", tc.verdict, got, tc.want)
		}
	}
}
