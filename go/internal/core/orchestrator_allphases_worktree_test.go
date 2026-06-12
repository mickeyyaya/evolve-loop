// orchestrator_allphases_worktree_test.go — CB.1 contract (concurrency campaign W4):
// EVERY dispatched phase runs with cwd = the cycle worktree.
//
// Pre-CB.1, runsInWorktree scoped the worktree cwd to source writers (tdd,
// build, writes_source user phases) + audit. Everything else dispatched with
// Worktree="" → cwd = the MAIN repo root, so a read-only phase's stray write
// (or a guard misfire — the cycle-280 inserted-phase fatal) landed in the live
// tree. CB.1 inverts the dispatch default: the worktree is provisioned at
// cycle start, so every phase gets cwd=worktree and the main tree is touched
// by no phase subprocess at all (the integrator alone writes main, CD track).
//
// CRITICAL discriminator these tests keep honest: cwd is NOT write permission.
// The write axis (role-gate + tree-diff guard + normalize) keys off
// worktreePhase / WorktreePhase and minted writes_source — that axis must be
// BYTE-IDENTICAL before and after CB.1. Only the cwd axis widens.
package core

import (
	"context"
	"testing"
)

// cb1Harness runs one happy-path cycle with a scripted worktree path and
// returns the runners map so tests can inspect every recorded PhaseRequest.
func cb1Harness(t *testing.T, wt *fakeWorktree) map[Phase]PhaseRunner {
	t.Helper()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	o := NewOrchestrator(st, led, runners, WithWorktreeProvisioner(wt))
	if _, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: t.TempDir(),
		GoalHash:    "cb1",
	}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	return runners
}

// TestCB1_EveryDispatchedPhaseCarriesWorktree: the CB.1 contract. Every phase
// that ran — scout, triage, ship, retro included, not just tdd/build/audit —
// must receive PhaseRequest.Worktree == the provisioned cycle worktree, so its
// subprocess cwd (and sandbox write surface) is the isolated checkout, never
// the live main tree.
func TestCB1_EveryDispatchedPhaseCarriesWorktree(t *testing.T) {
	wt := &fakeWorktree{path: t.TempDir()}
	runners := cb1Harness(t, wt)

	called := 0
	for phase, r := range runners {
		fr := r.(*fakeRunner)
		if fr.calls == 0 {
			continue // not every registered phase runs in a happy-path cycle
		}
		called++
		for i, req := range fr.requests {
			if req.Worktree != wt.path {
				t.Errorf("phase %s request[%d].Worktree=%q, want %q — "+
					"a phase dispatched without the cycle worktree runs cwd=main-tree "+
					"(the cycle-280 class CB.1 closes)", phase, i, req.Worktree, wt.path)
			}
		}
	}
	if called < 5 {
		t.Fatalf("harness ran only %d phases — too few to prove the all-phases contract", called)
	}
}

// TestCB1_WriteAxisUnchanged: widening the cwd axis must NOT widen the write
// axis. The role-gate, tree-diff guard, and build-commit normalize all key off
// worktreePhase / WorktreePhase — still exactly the source writers.
func TestCB1_WriteAxisUnchanged(t *testing.T) {
	t.Parallel()
	st := &fakeStorage{}
	o := NewOrchestrator(st, &fakeLedger{}, buildRunners(nil))

	writeCapable := map[Phase]bool{PhaseTDD: true, PhaseBuild: true}
	for _, p := range []Phase{PhaseIntent, PhaseScout, PhaseTriage, PhaseTDD,
		PhaseBuildPlanner, PhaseBuild, PhaseAudit, PhaseShip, PhaseRetro} {
		if got, want := o.worktreePhase(p), writeCapable[p]; got != want {
			t.Errorf("worktreePhase(%s)=%v, want %v — CB.1 must not move the write axis", p, got, want)
		}
		if got, want := WorktreePhase(p), writeCapable[p]; got != want {
			t.Errorf("WorktreePhase(%s)=%v, want %v — role-gate key must be untouched", p, got, want)
		}
	}
}

// TestCB1_ProvisioningFailureDegradesToEmptyWorktree: when worktree creation
// fails the cycle still runs (best-effort provisioning, role-gate blocks
// source writes loudly) and every phase dispatches with Worktree="" — the
// pre-existing degraded mode, unchanged by CB.1.
func TestCB1_ProvisioningFailureDegradesToEmptyWorktree(t *testing.T) {
	wt := &fakeWorktree{createErr: context.DeadlineExceeded}
	runners := cb1Harness(t, wt)

	for phase, r := range runners {
		fr := r.(*fakeRunner)
		for i, req := range fr.requests {
			if req.Worktree != "" {
				t.Errorf("phase %s request[%d].Worktree=%q, want \"\" when provisioning failed", phase, i, req.Worktree)
			}
		}
	}
}

// TestCB1_ResumePathCarriesWorktree: RunCycleFromPhase is a first-class
// dispatch surface — `evolve loop --resume` is the standard recovery path
// after ANY cycle failure — and it builds its PhaseRequest independently of
// the RunCycle loop. It must thread the persisted cs.ActiveWorktree into
// every resumed phase, or a resumed tdd/build runs cwd=main-tree: the exact
// cycle-280 class CB.1 closes, reopened only on resume (review BLOCK finding).
func TestCB1_ResumePathCarriesWorktree(t *testing.T) {
	t.Parallel()
	const wt = "/tmp/wt-resume-cycle-9"
	st := &fakeStorage{
		state: State{LastCycleNumber: 9},
		cycleState: CycleState{
			CycleID:        9,
			WorkspacePath:  t.TempDir(),
			ActiveWorktree: wt,
		},
	}
	runners := buildRunners(nil)
	o := NewOrchestrator(st, &fakeLedger{}, runners)
	if _, err := o.RunCycleFromPhase(context.Background(), CycleRequest{
		ProjectRoot: t.TempDir(),
	}, &ResumePoint{Phase: string(PhaseAudit), CycleID: 9}); err != nil {
		t.Fatalf("RunCycleFromPhase: %v", err)
	}

	called := 0
	for phase, r := range runners {
		fr := r.(*fakeRunner)
		if fr.calls == 0 {
			continue
		}
		called++
		for i, req := range fr.requests {
			if req.Worktree != wt {
				t.Errorf("resumed phase %s request[%d].Worktree=%q, want %q — "+
					"the resume path must thread the persisted ActiveWorktree", phase, i, req.Worktree, wt)
			}
		}
	}
	if called == 0 {
		t.Fatal("resume harness dispatched no phases — contract not exercised")
	}
}

// TestCB1_FailureLearningRetroCarriesWorktree: the out-of-band failure-
// learning retro (dispatched when a phase fails mid-cycle) builds its own
// PhaseRequest via retroRequest. Read-only, but the CB.1 invariant is "no
// phase subprocess has the main tree as cwd" — no exceptions, or the
// invariant stops being structural.
func TestCB1_FailureLearningRetroCarriesWorktree(t *testing.T) {
	t.Parallel()
	fl := failureLearningRequest{
		CycleRequest: CycleRequest{ProjectRoot: "/tmp/p"},
		Cycle:        7,
		Failed:       PhaseBuild,
		Err:          context.DeadlineExceeded,
		Attempt:      1,
		CycleState:   &CycleState{WorkspacePath: "/tmp/ws", ActiveWorktree: "/tmp/wt-fl"},
	}
	if got := fl.retroRequest("summary", "todo-1").Worktree; got != "/tmp/wt-fl" {
		t.Errorf("failure-learning retro Worktree=%q, want /tmp/wt-fl", got)
	}
}
