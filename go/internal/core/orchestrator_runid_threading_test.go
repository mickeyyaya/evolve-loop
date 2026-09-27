package core

import (
	"context"
	"testing"
)

func TestCB5_EveryDispatchedPhaseCarriesRunID(t *testing.T) {
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	runners := buildRunners(nil)
	o := NewOrchestrator(st, &fakeLedger{}, runners, WithWorktreeProvisioner(&fakeWorktree{path: t.TempDir()}))
	if _, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: t.TempDir(),
		GoalHash:    "cb5",
	}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	minted := st.cycleState.RunID
	if minted == "" {
		t.Fatal("precondition: RunCycle minted no RunID (CA.5 regression)")
	}
	for phase, r := range runners {
		fr := r.(*fakeRunner)
		for i, req := range fr.requests {
			if req.RunID != minted {
				t.Errorf("phase %s request[%d].RunID=%q, want %q — run identity must reach every dispatch", phase, i, req.RunID, minted)
			}
		}
	}
}

func TestCB5_ResumePathCarriesRunID(t *testing.T) {
	t.Parallel()
	const runID = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	st := &fakeStorage{
		state: State{LastCycleNumber: 9},
		cycleState: CycleState{
			CycleID:       9,
			WorkspacePath: t.TempDir(),
			RunID:         runID,
		},
	}
	runners := buildRunners(nil)
	o := NewOrchestrator(st, &fakeLedger{}, runners)
	if _, err := o.RunCycleFromPhase(context.Background(), CycleRequest{
		ProjectRoot: t.TempDir(),
	}, &ResumePoint{Phase: string(PhaseAudit), CycleID: 9}); err != nil {
		t.Fatalf("RunCycleFromPhase: %v", err)
	}
	for phase, r := range runners {
		fr := r.(*fakeRunner)
		for i, req := range fr.requests {
			if req.RunID != runID {
				t.Errorf("resumed phase %s request[%d].RunID=%q, want %q", phase, i, req.RunID, runID)
			}
		}
	}
}

func TestCB5_FailureLearningRetroCarriesRunID(t *testing.T) {
	t.Parallel()
	fl := failureLearningRequest{
		CycleRequest: CycleRequest{}, // retroRequest only copies the field; no I/O — a TempDir here is ceremony
		Cycle:        7,
		Failed:       PhaseBuild,
		Err:          context.DeadlineExceeded,
		Attempt:      1,
		CycleState:   &CycleState{WorkspacePath: "/tmp/ws", RunID: "01ARZ3NDEKTSV4RRFFQ69G5FAV"},
	}
	if got := fl.retroRequest("summary", "todo-1").RunID; got != "01ARZ3NDEKTSV4RRFFQ69G5FAV" {
		t.Errorf("failure-learning retro RunID=%q, want the cycle's run id", got)
	}
}
