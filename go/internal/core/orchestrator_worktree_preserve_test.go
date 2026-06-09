// orchestrator_worktree_preserve_test.go — RED contract for ship-failure
// worktree preservation (ADR-0039 §8, operator-approved 2026-06-07).
//
// D10 incident (domain-campaign cycles 7/12): a ship abort BEFORE the
// worktree-commit step let the deferred cycle cleanup prune the worktree with
// uncommitted, substantively-PASS work inside — cycle 7's work was lost
// entirely. The rule under test: when the cycle ends with an unresolved
// ship failure, the worktree is PRESERVED for recovery; it is only cleaned
// up when the cycle's ship eventually succeeds (or the operator runs an
// explicit `evolve cycle reset`).
package core

import (
	"context"
	"testing"
)

// failingShipRunner always fails with the configured ShipError.
type failingShipRunner struct{ err error }

func (r *failingShipRunner) Name() string { return "ship" }
func (r *failingShipRunner) Run(_ context.Context, _ PhaseRequest) (PhaseResponse, error) {
	return PhaseResponse{Phase: "ship", Verdict: VerdictFAIL}, r.err
}

// recoveringShipRunner fails the first call, then passes — exercising the
// "ship eventually succeeds after recovery" cleanup path.
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

// TestOrchestrator_ShipFailureAborts_PreservesWorktree: an unrecoverable
// (integrity) ship failure aborts the cycle — and the worktree must survive
// for operator/recovery triage instead of being pruned by the exit cleanup.
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

	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: "/tmp/p", GoalHash: "g"}); err == nil {
		t.Fatal("integrity ship failure must abort the cycle")
	}
	if len(wt.cleaned) != 0 {
		t.Fatalf("worktree pruned on ship failure (cleaned=%v) — audited work must be preserved for recovery", wt.cleaned)
	}
}

// TestOrchestrator_ShipRecoversThenSucceeds_CleansWorktree: when recovery
// retries ship and it succeeds, the normal exit cleanup applies — the
// preservation rule must not leak worktrees on eventually-successful cycles.
func TestOrchestrator_ShipRecoversThenSucceeds_CleansWorktree(t *testing.T) {
	t.Parallel()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	ship := &recoveringShipRunner{}
	runners[PhaseShip] = ship
	wt := &fakeWorktree{path: "/tmp/wt/cycle-1"}
	o := NewOrchestrator(st, led, runners, WithWorktreeProvisioner(wt))

	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: "/tmp/p", GoalHash: "g"}); err != nil {
		t.Fatalf("transient ship failure should recover: %v", err)
	}
	if ship.calls != 2 {
		t.Fatalf("ship calls = %d, want 2 (fail → retry)", ship.calls)
	}
	if len(wt.cleaned) != 1 || wt.cleaned[0] != "/tmp/wt/cycle-1" {
		t.Fatalf("worktree must be cleaned after the ship eventually succeeds; cleaned=%v", wt.cleaned)
	}
}
