package core

import (
	"context"
	"testing"
)

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

func TestCB1_EveryDispatchedPhaseCarriesWorktree(t *testing.T) {
	wt := &fakeWorktree{path: t.TempDir()}
	runners := cb1Harness(t, wt)

	called := 0
	for phase, r := range runners {
		fr := r.(*fakeRunner)
		if fr.calls == 0 {
			continue
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

func TestCB1_FailureLearningRetroCarriesWorktree(t *testing.T) {
	t.Parallel()
	fl := failureLearningRequest{
		CycleRequest: CycleRequest{}, // retroRequest only copies the field; no I/O — a TempDir here is ceremony
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
