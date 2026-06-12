// orchestrator_runid_threading_test.go — CB.5 contract (concurrency campaign
// W4), core half: the CA.5 run identity reaches every PhaseRequest, on every
// dispatch surface (RunCycle loop, resume, failure-learning retro), so the
// bridge can mint run-scoped tmux session names (evolve-bridge-r<runid8>-…)
// and the per-run session registry records the right owner. Without the
// threading, two concurrent runs' sessions are distinguishable only by
// pid+timestamp — nothing an observer or reaper can ASSERT on (CB.6).
package core

import (
	"context"
	"testing"
)

// TestCB5_EveryDispatchedPhaseCarriesRunID: the RunCycle loop stamps the
// minted run id into every PhaseRequest.
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

// TestCB5_ResumePathCarriesRunID: RunCycleFromPhase threads the PERSISTED
// run id (resume reuses the run-record identity, CA.5) into resumed phases.
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

// TestCB5_FailureLearningRetroCarriesRunID: the third construction site
// (the CB.1 review taught us to sweep them all).
func TestCB5_FailureLearningRetroCarriesRunID(t *testing.T) {
	t.Parallel()
	fl := failureLearningRequest{
		CycleRequest: CycleRequest{ProjectRoot: "/tmp/p"},
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
