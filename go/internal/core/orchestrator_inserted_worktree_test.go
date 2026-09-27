package core

import (
	"context"
	"testing"
)

// writableMintPlanJSON's mint block omits writes_source. parsePhasePlan is the
// real advisor-output parser, so the test exercises the genuine
// mint→register→dispatch seam, not a mock.
const writableMintPlanJSON = `[{"phase":"test-amplification","run":true,"justification":"amplify adversarial tests for this cycle","mint":{"prompt":"You amplify the test suite with adversarial fault cases.","tier":"balanced","cli":"claude"}}]`

const readOnlyMintPlanJSON = `[{"phase":"lint-advisor","run":true,"justification":"read-only style advisory, writes nothing","mint":{"prompt":"You report lint findings; you never edit files.","tier":"fast","cli":"claude","writes_source":false}}]`

func TestInsertedPhaseWritableInheritsWorktree(t *testing.T) {
	t.Parallel()
	o := mintOrchestrator(t, fakeMinter{})
	plan, err := parsePhasePlan(writableMintPlanJSON)
	if err != nil {
		t.Fatalf("parsePhasePlan(writable mint): %v", err)
	}
	o.registerMintedPhases(plan)

	if _, ok := o.runners[Phase("test-amplification")]; !ok {
		t.Fatal("precondition: minted phase was not registered into runners")
	}
	if !o.worktreePhase(Phase("test-amplification")) {
		t.Errorf("inserted phase 'test-amplification' is not write-capable (worktreePhase=false) — " +
			"a minted phase that does not opt out of source writes must default to write-capable (cycle-280).")
	}
}

func TestInsertedReadOnlyPhaseDoesNotGetWorktree(t *testing.T) {
	t.Parallel()
	o := mintOrchestrator(t, fakeMinter{})
	plan, err := parsePhasePlan(readOnlyMintPlanJSON)
	if err != nil {
		t.Fatalf("parsePhasePlan(read-only mint): %v", err)
	}
	o.registerMintedPhases(plan)

	if _, ok := o.runners[Phase("lint-advisor")]; !ok {
		t.Fatal("precondition: minted phase was not registered into runners")
	}
	if o.worktreePhase(Phase("lint-advisor")) {
		t.Errorf("inserted phase 'lint-advisor' explicitly set writes_source:false but is write-capable " +
			"(worktreePhase=true) — an explicit read-only opt-out must be honoured on the role-gate axis.")
	}
}

type fatalBuildRunner struct{ err error }

func (r *fatalBuildRunner) Name() string { return "build" }
func (r *fatalBuildRunner) Run(_ context.Context, _ PhaseRequest) (PhaseResponse, error) {
	return PhaseResponse{Phase: "build", Verdict: VerdictFAIL}, r.err
}

func TestAbortCleanupPreservesWorktreeDiff(t *testing.T) {
	t.Parallel()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	runners[PhaseBuild] = &fatalBuildRunner{err: errTest("build phase aborted mid-cycle")}
	wt := &fakeWorktree{path: "/tmp/wt/cycle-1"}
	o := NewOrchestrator(st, led, runners, WithWorktreeProvisioner(wt))

	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g"}); err == nil {
		t.Fatal("a fatal build phase must abort the cycle with an error")
	}
	if len(wt.cleaned) != 0 {
		t.Fatalf("RED: worktree pruned on abnormal mid-cycle abort (cleaned=%v) — uncommitted builder work "+
			"must be preserved for recovery (`evolve loop --resume` / `evolve cycle reset`), exactly as the "+
			"ship-failure path preserves it. This silent delete is the cycle-280 data-loss bug.", wt.cleaned)
	}
}
