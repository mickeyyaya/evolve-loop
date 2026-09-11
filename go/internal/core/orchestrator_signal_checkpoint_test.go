package core_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mickeyyaya/evolve-loop/go/internal/checkpoint"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

type cancelingSignalRunner struct {
	name   string
	cancel context.CancelFunc
}

func (r *cancelingSignalRunner) Name() string { return r.name }

func (r *cancelingSignalRunner) Run(ctx context.Context, _ core.PhaseRequest) (core.PhaseResponse, error) {
	r.cancel()
	<-ctx.Done()
	return core.PhaseResponse{Phase: r.name}, ctx.Err()
}

type countingSignalRunner struct {
	name  string
	calls int
}

func (r *countingSignalRunner) Name() string { return r.name }

func (r *countingSignalRunner) Run(_ context.Context, req core.PhaseRequest) (core.PhaseResponse, error) {
	r.calls++
	return core.PhaseResponse{Phase: r.name, Verdict: core.VerdictPASS, ArtifactsDir: req.Workspace}, nil
}

func TestRunCycle_InterruptCheckpointsActivePhaseWithoutRetrospective(t *testing.T) {
	savedPhaseCheckpointer := core.PhaseBoundaryCheckpointer
	savedResumeCheckpointer := core.ResumeBoundaryCheckpointer
	t.Cleanup(func() {
		core.PhaseBoundaryCheckpointer = savedPhaseCheckpointer
		core.ResumeBoundaryCheckpointer = savedResumeCheckpointer
	})

	core.PhaseBoundaryCheckpointer = func(core.CycleState, string, time.Time) error { return nil }
	var checkpoints []core.CycleState
	core.ResumeBoundaryCheckpointer = func(cs core.CycleState, _ string, _ time.Time) error {
		checkpoints = append(checkpoints, cs)
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	retro := &countingSignalRunner{name: string(core.PhaseRetro)}
	runners := newRunners(map[core.Phase]core.PhaseRunner{
		core.PhaseTriage: &cancelingSignalRunner{name: string(core.PhaseTriage), cancel: cancel},
		core.PhaseRetro:  retro,
	})
	orch, _, _ := newTestOrchestrator(t, runners)

	_, err := orch.RunCycle(ctx, core.CycleRequest{
		ProjectRoot: t.TempDir(),
		GoalHash:    "interrupt-active-phase",
		Context:     map[string]string{"commit_message": "test interrupt checkpoint"},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RunCycle error = %v, want context.Canceled", err)
	}
	if retro.calls != 0 {
		t.Fatalf("retrospective calls = %d, want 0 for an operator interrupt", retro.calls)
	}
	if len(checkpoints) != 1 {
		t.Fatalf("interrupt checkpoints = %d, want exactly 1", len(checkpoints))
	}

	checkpoint := checkpoints[0]
	if checkpoint.Phase != string(core.PhaseTriage) {
		t.Errorf("resume phase = %q, want active phase %q", checkpoint.Phase, core.PhaseTriage)
	}
	if !containsStr(checkpoint.CompletedPhases, string(core.PhaseScout)) {
		t.Errorf("completed phases = %v, want completed scout retained", checkpoint.CompletedPhases)
	}
	if containsStr(checkpoint.CompletedPhases, string(core.PhaseTriage)) {
		t.Errorf("completed phases = %v, interrupted triage must remain incomplete", checkpoint.CompletedPhases)
	}
}

func TestRunCycle_InterruptCheckpointSelectsActivePhaseOnResume(t *testing.T) {
	root := t.TempDir()
	seedCycleStateFile(t, root)

	ctx, cancel := context.WithCancel(context.Background())
	runners := newRunners(map[core.Phase]core.PhaseRunner{
		core.PhaseTriage: &cancelingSignalRunner{name: string(core.PhaseTriage), cancel: cancel},
	})
	orch, _, _ := newTestOrchestrator(t, runners)

	_, err := orch.RunCycle(ctx, core.CycleRequest{
		ProjectRoot: root,
		GoalHash:    "durable-interrupt-checkpoint",
		Context:     map[string]string{"commit_message": "test durable interrupt checkpoint"},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RunCycle error = %v, want context.Canceled", err)
	}

	resume, err := core.LoadResumeState(context.Background(), root, filepath.Join(root, ".evolve"), core.ResumeOptions{})
	if err != nil {
		t.Fatalf("LoadResumeState: %v", err)
	}
	if resume.Phase != string(core.PhaseTriage) {
		t.Errorf("resume phase = %q, want interrupted phase %q", resume.Phase, core.PhaseTriage)
	}
	if resume.Reason != "operator-requested" {
		t.Errorf("resume reason = %q, want operator-requested", resume.Reason)
	}
	if !containsStr(resume.CompletedPhases, string(core.PhaseScout)) {
		t.Errorf("completed phases = %v, want completed scout retained", resume.CompletedPhases)
	}
	if containsStr(resume.CompletedPhases, string(core.PhaseTriage)) {
		t.Errorf("completed phases = %v, interrupted triage must remain incomplete", resume.CompletedPhases)
	}
}
