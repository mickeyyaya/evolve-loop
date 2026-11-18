package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
)

type wallAtCallRunner struct {
	*ladderProbe
	wallAt int
}

func (w *wallAtCallRunner) Run(ctx context.Context, req PhaseRequest) (PhaseResponse, error) {
	resp, err := w.ladderProbe.Run(ctx, req)
	if err == nil && w.wallAt > 0 && len(w.requests) == w.wallAt {
		return PhaseResponse{}, wrapTransient(85)
	}
	return resp, err
}

type resumedLadderCycle struct {
	o       *Orchestrator
	st      *fakeStorage
	request CycleRequest
	build   *fakeRunner
	tdd     *fakeRunner
	audit   *fakeRunner
}

func newResumedLadderCycle(t *testing.T, stage string, checkpoint CycleState, tddRunner PhaseRunner, reviewer DeliverableReviewer) resumedLadderCycle {
	t.Helper()
	overrides := map[Phase]PhaseRunner{}
	if tddRunner != nil {
		overrides[PhaseTDD] = tddRunner
	}
	return newResumedLadderCycleWith(t, stage, checkpoint, overrides, reviewer)
}

func newResumedLadderCycleWith(t *testing.T, stage string, checkpoint CycleState, overrides map[Phase]PhaseRunner, reviewer DeliverableReviewer) resumedLadderCycle {
	t.Helper()
	root := initDossierRepo(t)
	writeCLIProfile(t, root, "tdd-engineer", "agy-tmux", nil)
	writeCLIProfile(t, root, "builder", "agy-tmux", nil)
	if checkpoint.WorkspacePath == "" {
		checkpoint.WorkspacePath = t.TempDir()
	}
	checkpoint.CycleID = 9
	checkpoint.ActiveWorktree = t.TempDir()
	st := &fakeStorage{state: State{LastCycleNumber: 9}, cycleState: checkpoint}
	runners := buildRunners(nil)
	cycle := resumedLadderCycle{st: st, request: CycleRequest{ProjectRoot: root, GoalHash: "g"}}
	for phase, runner := range overrides {
		runners[phase] = runner
	}
	cycle.tdd, _ = runners[PhaseTDD].(*fakeRunner)
	cycle.audit, _ = runners[PhaseAudit].(*fakeRunner)
	cycle.build, _ = runners[PhaseBuild].(*fakeRunner)
	opts := []Option{
		WithWorktreeProvisioner(&fakeWorktree{path: checkpoint.ActiveWorktree}),
		WithRetryConfig(policy.RetryConfig{ContractCorrectionRetries: 1}),
		WithWorkflowConfig(ladderWorkflow(stage)),
	}
	if reviewer != nil {
		opts = append(opts, WithReviewer(reviewer))
	}
	cycle.o = NewOrchestrator(st, &fakeLedger{}, runners, opts...)
	return cycle
}

func (c resumedLadderCycle) resumeAt(phase Phase) error {
	_, err := c.o.RunCycleFromPhase(context.Background(), c.request, &ResumePoint{Phase: string(phase), CycleID: 9})
	return err
}

func TestResumedCycle_ContinuesAtItsRecordedRung(t *testing.T) {
	t.Parallel()
	recorded := &LadderState{Rung: string(retryladder.R2), Tier: "deep", Effort: "high", Fingerprints: []string{"cover_changed"}}
	for _, tc := range []struct {
		name     string
		stage    string
		ladder   *LadderState
		wantRung retryladder.Rung
	}{
		{"enforced ladder at R2", policy.RetryLadderEnforce, recorded, retryladder.R2},
		{"enforced ladder at R0", policy.RetryLadderEnforce, &LadderState{Rung: string(retryladder.R0)}, ""},
		{"enforced and never recorded", policy.RetryLadderEnforce, nil, ""},
		{"shadow ladder at R2", policy.RetryLadderShadow, recorded, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cycle := newResumedLadderCycle(t, tc.stage, CycleState{Ladder: tc.ladder}, nil, nil)
			if err := cycle.resumeAt(PhaseTDD); err != nil {
				t.Fatalf("resume: %v", err)
			}
			if len(cycle.tdd.requests) == 0 {
				t.Fatal("the resumed phase was never dispatched")
			}
			req := cycle.tdd.requests[0]
			if tc.wantRung == "" {
				if req.Ladder != nil || req.Assignment.Effort != "" {
					t.Fatalf("the dispatch must stay unchanged: ladder %+v effort %q", req.Ladder, req.Assignment.Effort)
				}
				return
			}
			if req.Ladder == nil || req.Ladder.Rung != tc.wantRung || req.Ladder.Tier != "deep" || req.Assignment.Effort != "high" {
				t.Fatalf("the resumed phase did not continue at its recorded rung: ladder %+v effort %q", req.Ladder, req.Assignment.Effort)
			}
		})
	}
}

func TestResumedCorrection_TheRaisedRungReachesTheNextPhase(t *testing.T) {
	t.Parallel()
	probe := &wallAtCallRunner{ladderProbe: newLadderProbe(2)}
	probe.phase = string(PhaseTDD)
	cycle := newResumedLadderCycle(t, policy.RetryLadderEnforce, CycleState{}, probe, probe)
	if err := cycle.resumeAt(PhaseTDD); err != nil {
		t.Fatalf("resume: %v", err)
	}
	raised := probe.requests[len(probe.requests)-1]
	if raised.Ladder == nil || raised.Ladder.Context != retryladder.ContextFresh || raised.Ladder.Rung == retryladder.R0 {
		t.Fatalf("the last tdd correction is not a raised try: %+v", raised.Ladder)
	}
	if len(cycle.build.requests) == 0 {
		t.Fatal("the build phase was never dispatched")
	}
	next := cycle.build.requests[0]
	if next.Ladder == nil || next.Ladder.Rung != raised.Ladder.Rung || next.Ladder.Tier != raised.Ladder.Tier || next.Assignment.Effort != raised.Assignment.Effort {
		t.Fatalf("the next phase of the resumed run left the raised rung: build %+v effort %q, raised %+v effort %q",
			next.Ladder, next.Assignment.Effort, raised.Ladder, raised.Assignment.Effort)
	}
}

func TestResumedCorrection_AQuotaWallPausesAtTheRaisedRungAndTheNextResumeContinuesThere(t *testing.T) {
	var checkpoints []CycleState
	previous := QuotaBoundaryCheckpointer
	t.Cleanup(func() { QuotaBoundaryCheckpointer = previous })
	QuotaBoundaryCheckpointer = func(cs CycleState, _ string, _ time.Time) error {
		checkpoints = append(checkpoints, cs)
		return nil
	}

	probe := &wallAtCallRunner{ladderProbe: newLadderProbe(0), wallAt: 3}
	probe.phase = string(PhaseTDD)
	paused := newResumedLadderCycle(t, policy.RetryLadderEnforce, CycleState{}, probe, probe)
	if err := paused.resumeAt(PhaseTDD); !errors.Is(err, ErrAllFamiliesExhausted) {
		t.Fatalf("the wall must pause the resumed run: %v", err)
	}
	raised := probe.requests[len(probe.requests)-1]
	if raised.Ladder == nil || raised.Ladder.Rung == retryladder.R0 {
		t.Fatalf("the correction that met the wall is not a raised try: %+v", raised.Ladder)
	}
	if len(checkpoints) != 1 {
		t.Fatalf("the pause wrote %d quota checkpoints, want 1", len(checkpoints))
	}
	saved := checkpoints[0].Ladder
	if saved == nil || saved.Rung != string(raised.Ladder.Rung) || saved.Tier != raised.Ladder.Tier || saved.Effort != raised.Assignment.Effort {
		t.Fatalf("the quota checkpoint lost the rung of the correction loop: saved %+v, raised try %+v effort %q", saved, raised.Ladder, raised.Assignment.Effort)
	}

	resumed := newResumedLadderCycle(t, policy.RetryLadderEnforce, CycleState{Ladder: saved, WorkspacePath: checkpoints[0].WorkspacePath}, nil, nil)
	if err := resumed.resumeAt(PhaseTDD); err != nil {
		t.Fatalf("second resume: %v", err)
	}
	again := resumed.tdd.requests[0]
	if again.Ladder == nil || again.Ladder.Rung != raised.Ladder.Rung || again.Assignment.Effort != raised.Assignment.Effort {
		t.Fatalf("the resume after the pause did not continue at the recorded rung: %+v effort %q", again.Ladder, again.Assignment.Effort)
	}
}

func TestResumedCorrection_ShadowQuotaPauseLeavesTheNextResumeDispatchUnchanged(t *testing.T) {
	var checkpoints []CycleState
	previous := QuotaBoundaryCheckpointer
	t.Cleanup(func() { QuotaBoundaryCheckpointer = previous })
	QuotaBoundaryCheckpointer = func(cs CycleState, _ string, _ time.Time) error {
		checkpoints = append(checkpoints, cs)
		return nil
	}

	probe := &wallAtCallRunner{ladderProbe: newLadderProbe(0), wallAt: 2}
	probe.phase = string(PhaseTDD)
	paused := newResumedLadderCycle(t, policy.RetryLadderShadow, CycleState{}, probe, probe)
	if err := paused.resumeAt(PhaseTDD); !errors.Is(err, ErrAllFamiliesExhausted) {
		t.Fatalf("the wall must pause the resumed run: %v", err)
	}
	if len(checkpoints) != 1 {
		t.Fatalf("the pause wrote %d quota checkpoints, want 1", len(checkpoints))
	}
	resumed := newResumedLadderCycle(t, policy.RetryLadderShadow, CycleState{Ladder: checkpoints[0].Ladder, WorkspacePath: checkpoints[0].WorkspacePath}, nil, nil)
	if err := resumed.resumeAt(PhaseTDD); err != nil {
		t.Fatalf("second resume: %v", err)
	}
	if req := resumed.tdd.requests[0]; req.Ladder != nil || req.Assignment.Effort != "" {
		t.Fatalf("a shadow ladder must not change a dispatch: %+v effort %q", req.Ladder, req.Assignment.Effort)
	}
}
