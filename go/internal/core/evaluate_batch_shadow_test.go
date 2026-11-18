package core

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func shadowBatchRun(t *testing.T, stage config.Stage) (*fakeRunner, *fakeRunner, []signalcenter.Event, error) {
	t.Helper()
	a := &fakeRunner{name: "check-a"}
	b := &fakeRunner{name: "check-b"}
	o, _ := evaluateBatchRunCycleE4(t, a, b)
	o.cfg.ParallelEvaluate = stage
	c, got := recordingCenter()
	o.signals = c
	_, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g", DisableWorkspaceGuard: true})
	c.Flush()
	return a, b, eventsWithCode(*got, signalcenter.CodeParallelEvaluateShadow), err
}

func TestOrchestrator_ParallelEvaluateShadowRecordsBatch(t *testing.T) {
	t.Parallel()

	a, b, events, err := shadowBatchRun(t, config.StageShadow)

	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if a.calls != 1 || b.calls != 1 {
		t.Fatalf("shadow must still dispatch each member once, serially: a=%d b=%d", a.calls, b.calls)
	}
	if len(events) != 1 {
		t.Fatalf("PARALLEL_EVALUATE_SHADOW events = %d, want 1", len(events))
	}
	if got := events[0].Fields["batch"]; got != "check-a,check-b" {
		t.Errorf("fields.batch = %q, want check-a,check-b", got)
	}
	if events[0].Module != signalcenter.ModuleParallelEval || events[0].Severity != signalcenter.SeverityInfo {
		t.Errorf("event = %+v, want an INFO event of module parallel.evaluate", events[0])
	}
}

func TestShadowIsNotOff(t *testing.T) {
	t.Parallel()

	for _, stage := range []config.Stage{config.StageOff, config.StageEnforce} {
		_, _, events, err := shadowBatchRun(t, stage)
		if err != nil {
			t.Fatalf("stage %v: %v", stage, err)
		}
		if len(events) != 0 {
			t.Errorf("stage %v emitted %d PARALLEL_EVALUATE_SHADOW events, want 0: only shadow forecasts", stage, len(events))
		}
	}
	_, _, shadowEvents, err := shadowBatchRun(t, config.StageShadow)
	if err != nil || len(shadowEvents) == 0 {
		t.Errorf("shadow emitted nothing (err %v): shadow is off", err)
	}
}

type overlapRunner struct {
	name    string
	active  *atomic.Int32
	overlap *atomic.Bool
}

func (r *overlapRunner) Name() string { return r.name }
func (r *overlapRunner) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	if r.active.Add(1) > 1 {
		r.overlap.Store(true)
	}
	time.Sleep(40 * time.Millisecond)
	r.active.Add(-1)
	return PhaseResponse{Phase: r.name, Verdict: VerdictPASS, ArtifactsDir: req.Workspace}, nil
}

func overlapRun(t *testing.T, stage config.Stage) bool {
	t.Helper()
	var active atomic.Int32
	var overlap atomic.Bool
	a := &overlapRunner{name: "check-a", active: &active, overlap: &overlap}
	b := &overlapRunner{name: "check-b", active: &active, overlap: &overlap}
	o, _ := evaluateBatchRunCycleE4(t, a, b)
	o.cfg.ParallelEvaluate = stage
	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g", DisableWorkspaceGuard: true}); err != nil {
		t.Fatalf("stage %v: %v", stage, err)
	}
	return overlap.Load()
}

func TestShadowDispatchesTheBatchSerially(t *testing.T) {
	t.Parallel()

	if !overlapRun(t, config.StageEnforce) {
		t.Fatal("control: enforce must run the batch members concurrently, or the detector proves nothing")
	}
	if overlapRun(t, config.StageShadow) {
		t.Error("shadow ran the batch members concurrently: shadow must dispatch serially and decide nothing")
	}
}
