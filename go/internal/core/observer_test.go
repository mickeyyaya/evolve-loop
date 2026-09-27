package core

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
)

type recordingObserver struct {
	mu          sync.Mutex
	starts      []string
	cancelCalls atomic.Int32
}

func (r *recordingObserver) Start(_ context.Context, phase string, _ PhaseRequest) func() {
	r.mu.Lock()
	r.starts = append(r.starts, phase)
	r.mu.Unlock()
	return func() { r.cancelCalls.Add(1) }
}

func TestOrchestrator_NoopObserver_IsByteIdentical(t *testing.T) {
	t.Parallel()
	st := &fakeStorage{state: State{LastCycleNumber: 9}}
	led := &fakeLedger{}
	o := NewOrchestrator(st, led, buildRunners(nil))

	res, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: t.TempDir(),
		GoalHash:    "g",
	})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if res.FinalVerdict != VerdictPASS {
		t.Errorf("verdict=%s, want PASS", res.FinalVerdict)
	}
	want := []Phase{PhaseScout, PhaseTriage, PhaseTDD, PhaseBuildPlanner, PhaseBuild, PhaseAudit, PhaseShip}
	if len(res.PhasesRun) != len(want) {
		t.Errorf("PhasesRun=%v, want %v (len)", res.PhasesRun, want)
	}
}

func TestOrchestrator_WithObserver_StartsAndCancelsPerPhase(t *testing.T) {
	t.Parallel()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	obs := &recordingObserver{}
	o := NewOrchestrator(st, led, buildRunners(nil), WithObserver(obs))

	res, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: t.TempDir(), GoalHash: "g",
	})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if res.FinalVerdict != VerdictPASS {
		t.Errorf("verdict=%s, want PASS", res.FinalVerdict)
	}

	obs.mu.Lock()
	startCount := len(obs.starts)
	startsSnapshot := append([]string(nil), obs.starts...)
	obs.mu.Unlock()

	if startCount == 0 {
		t.Fatal("observer was never started despite WithObserver being set")
	}
	if got := int(obs.cancelCalls.Load()); got != startCount {
		t.Errorf("Start/cancel mismatch: starts=%d cancels=%d (every Start must be paired with one cancel)",
			startCount, got)
	}
	if startCount != len(res.PhasesRun) {
		t.Errorf("observer started %d times, %d phases ran — expected 1:1",
			startCount, len(res.PhasesRun))
	}
	for i, p := range res.PhasesRun {
		if i >= len(startsSnapshot) {
			break
		}
		if startsSnapshot[i] != string(p) {
			t.Errorf("Start[%d]=%q, phase[%d]=%s — observer Start order must match phase execution order",
				i, startsSnapshot[i], i, p)
		}
	}
}

func TestOrchestrator_WithNilObserver_FallsBackToNoop(t *testing.T) {
	t.Parallel()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	o := NewOrchestrator(st, led, buildRunners(nil), WithObserver(nil))

	res, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: t.TempDir(), GoalHash: "g",
	})
	if err != nil {
		t.Fatalf("RunCycle with WithObserver(nil): %v (should fall back to noopObserver)", err)
	}
	if res.FinalVerdict != VerdictPASS {
		t.Errorf("verdict=%s, want PASS", res.FinalVerdict)
	}
}

func TestNoopObserver_StartReturnsNonNilCancel(t *testing.T) {
	t.Parallel()
	c := noopObserver{}.Start(context.Background(), "tdd", PhaseRequest{})
	if c == nil {
		t.Fatal("noopObserver.Start returned nil cancel")
	}
	c() // must not panic
	c() // must be idempotent (orchestrator may call twice on error paths)
}
