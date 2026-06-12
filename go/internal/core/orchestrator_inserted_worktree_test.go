// orchestrator_inserted_worktree_test.go — RED contract for the cycle-280 P0:
// advisor-INSERTED (minted) write-capable phases dispatched with Worktree=""
// because the mint template defaults writes_source:false, so worktreePhase /
// runsInWorktree returned false → phaseWorktree="" → the tree-diff guard fired
// cycle-fatal AND the abort cleanup deleted the uncommitted worktree (all
// builder output lost). Two HIGH inbox items (2026-06-10T19:35Z/19:36Z) and the
// scout-281 P0 carryover mandate this fix.
//
// The contract these tests pin (implementation-agnostic — they assert the
// observable runsInWorktree / Cleanup behaviour, not any field type):
//
//  1. An inserted phase the advisor mints WITHOUT explicitly opting out of
//     source writes DEFAULTS to write-capable, so it inherits the cycle
//     worktree (runsInWorktree == true). A minted phase that gets no worktree
//     is the exact cycle-280 fatal.
//  2. A minted phase that EXPLICITLY declares writes_source:false stays
//     read-only and gets NO worktree (the discriminator: the fix must not
//     blanket-grant worktrees to every mint, only default the unspecified
//     ones to write-capable). Distinguishing "unspecified" from "explicit
//     false" is what forces the real fix (a tri-state mint flag) rather than a
//     no-op.
//  3. On an ABNORMAL mid-cycle abort (phase-fatal / guard-abort), the worktree
//     is PRESERVED, never pruned — so uncommitted builder work survives for
//     recovery, the way the ship-failure path already preserves it.
package core

import (
	"context"
	"testing"
)

// writableMintPlanJSON is a realistic advisor plan that inserts a write-capable
// phase the SAME way cycle-280's failing advisor did: a mint block that does NOT
// set writes_source. Per the cycle-280 forensic this must still inherit the
// worktree. parsePhasePlan is the real advisor-output parser, so the test
// exercises the genuine mint→register→dispatch seam.
const writableMintPlanJSON = `[{"phase":"test-amplification","run":true,"justification":"amplify adversarial tests for this cycle","mint":{"prompt":"You amplify the test suite with adversarial fault cases.","tier":"balanced","cli":"claude"}}]`

// readOnlyMintPlanJSON inserts a phase that EXPLICITLY opts out of source writes.
// It must remain read-only (no worktree) — the negative case that keeps the fix
// honest.
const readOnlyMintPlanJSON = `[{"phase":"lint-advisor","run":true,"justification":"read-only style advisory, writes nothing","mint":{"prompt":"You report lint findings; you never edit files.","tier":"fast","cli":"claude","writes_source":false}}]`

// TestInsertedPhaseWritableInheritsWorktree: an advisor-minted phase that does
// not explicitly mark itself read-only must dispatch with cwd=worktree. RED
// today because the mint default is writes_source:false → runsInWorktree false →
// Worktree="" → tree-diff guard fatal (cycle-280).
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
	// Post-CB.1 the worktree CWD is universal (every phase dispatches with it —
	// see TestCB1_EveryDispatchedPhaseCarriesWorktree), so the cycle-280 fatal
	// class is closed structurally. What this test still pins is the WRITE axis:
	// a mint that does not opt out of source writes must be write-capable, or
	// the role-gate denies its edits and the phase stalls.
	if !o.worktreePhase(Phase("test-amplification")) {
		t.Errorf("inserted phase 'test-amplification' is not write-capable (worktreePhase=false) — " +
			"a minted phase that does not opt out of source writes must default to write-capable (cycle-280).")
	}
}

// TestInsertedReadOnlyPhaseDoesNotGetWorktree: the discriminator. A minted phase
// that EXPLICITLY sets writes_source:false must stay read-only and get no
// worktree. This guards against an over-broad fix that hands every mint a
// worktree — the writable-default of test 1 must coexist with an honoured
// explicit opt-out, which is only expressible if the fix distinguishes
// "unspecified" from "explicit false".
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
	// Post-CB.1 every phase gets the worktree as CWD — including this one; that
	// is deliberate and harmless (cwd ≠ write permission). What the explicit
	// writes_source:false opt-out must still buy is the WRITE axis: the
	// role-gate must see this mint as a non-writer.
	if o.worktreePhase(Phase("lint-advisor")) {
		t.Errorf("inserted phase 'lint-advisor' explicitly set writes_source:false but is write-capable " +
			"(worktreePhase=true) — an explicit read-only opt-out must be honoured on the role-gate axis.")
	}
}

// fatalBuildRunner always returns an error, modelling a non-recoverable
// mid-cycle phase abort (the class that includes the cycle-280 guard-fatal).
type fatalBuildRunner struct{ err error }

func (r *fatalBuildRunner) Name() string { return "build" }
func (r *fatalBuildRunner) Run(_ context.Context, _ PhaseRequest) (PhaseResponse, error) {
	return PhaseResponse{Phase: "build", Verdict: VerdictFAIL}, r.err
}

// TestAbortCleanupPreservesWorktreeDiff: when a cycle ends ABNORMALLY before
// ship (here a fatal build phase — the same abort class as the cycle-280
// guard-fatal), the worktree holding uncommitted builder work must be PRESERVED
// for recovery, never pruned by the exit cleanup. RED today: the abort-cleanup
// defer only preserves on ship failure, so a mid-cycle phase-fatal silently
// deletes the worktree and its uncommitted diff (cycle-280 data loss).
func TestAbortCleanupPreservesWorktreeDiff(t *testing.T) {
	t.Parallel()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	runners[PhaseBuild] = &fatalBuildRunner{err: errTest("build phase aborted mid-cycle")}
	wt := &fakeWorktree{path: "/tmp/wt/cycle-1"}
	o := NewOrchestrator(st, led, runners, WithWorktreeProvisioner(wt))

	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: "/tmp/p", GoalHash: "g"}); err == nil {
		t.Fatal("a fatal build phase must abort the cycle with an error")
	}
	if len(wt.cleaned) != 0 {
		t.Fatalf("RED: worktree pruned on abnormal mid-cycle abort (cleaned=%v) — uncommitted builder work "+
			"must be preserved for recovery (`evolve loop --resume` / `evolve cycle reset`), exactly as the "+
			"ship-failure path preserves it. This silent delete is the cycle-280 data-loss bug.", wt.cleaned)
	}
}
