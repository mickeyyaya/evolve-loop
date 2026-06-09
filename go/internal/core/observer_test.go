package core

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
)

// recordingObserver is an Observer test double that counts Start +
// cancel calls per phase, so tests can assert the orchestrator
// invokes the lifecycle exactly once per phase in the correct order.
type recordingObserver struct {
	mu          sync.Mutex
	starts      []string // phase names in Start-call order
	cancelCalls atomic.Int32
}

func (r *recordingObserver) Start(_ context.Context, phase string, _ PhaseRequest) func() {
	r.mu.Lock()
	r.starts = append(r.starts, phase)
	r.mu.Unlock()
	return func() { r.cancelCalls.Add(1) }
}

// TestOrchestrator_NoopObserver_IsByteIdentical is the ADR-0030
// pre-opt-in contract: when the operator doesn't pass WithObserver,
// the orchestrator runs every phase to completion exactly like the
// pre-fix cycle — no observer-related behavior observable.
func TestOrchestrator_NoopObserver_IsByteIdentical(t *testing.T) {
	t.Parallel()
	st := &fakeStorage{state: State{LastCycleNumber: 9}}
	led := &fakeLedger{}
	o := NewOrchestrator(st, led, buildRunners(nil))
	// Explicitly NO WithObserver call.

	res, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: "/tmp/p",
		GoalHash:    "g",
	})
	if err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if res.FinalVerdict != VerdictPASS {
		t.Errorf("verdict=%s, want PASS", res.FinalVerdict)
	}
	// All phases ran — the noopObserver didn't disturb anything.
	want := []Phase{PhaseScout, PhaseTriage, PhaseTDD, PhaseBuildPlanner, PhaseBuild, PhaseAudit, PhaseShip}
	if len(res.PhasesRun) != len(want) {
		t.Errorf("PhasesRun=%v, want %v (len)", res.PhasesRun, want)
	}
}

// TestOrchestrator_WithObserver_StartsAndCancelsPerPhase proves the
// happy path: an injected observer's Start is called before each
// runner.Run and the returned cancel is invoked after. The Start
// count must equal the cancel count must equal the phase count —
// every Start gets paired with exactly one cancel.
func TestOrchestrator_WithObserver_StartsAndCancelsPerPhase(t *testing.T) {
	t.Parallel()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	obs := &recordingObserver{}
	o := NewOrchestrator(st, led, buildRunners(nil), WithObserver(obs))

	res, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: "/tmp/p", GoalHash: "g",
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
	// Observer must see EVERY phase that ran (not just the first).
	// PhasesRun records each phase; observer.Start should mirror it.
	if startCount != len(res.PhasesRun) {
		t.Errorf("observer started %d times, %d phases ran — expected 1:1",
			startCount, len(res.PhasesRun))
	}
	// Order: observer Start must match phase execution order.
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

// TestOrchestrator_WithNilObserver_FallsBackToNoop pins the
// WithObserver guard at the option function: passing nil must NOT
// overwrite the noopObserver default (a nil observer panics on first
// Start call).
func TestOrchestrator_WithNilObserver_FallsBackToNoop(t *testing.T) {
	t.Parallel()
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	o := NewOrchestrator(st, led, buildRunners(nil), WithObserver(nil))

	res, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: "/tmp/p", GoalHash: "g",
	})
	if err != nil {
		t.Fatalf("RunCycle with WithObserver(nil): %v (should fall back to noopObserver)", err)
	}
	if res.FinalVerdict != VerdictPASS {
		t.Errorf("verdict=%s, want PASS", res.FinalVerdict)
	}
}

// TestNoopObserver_StartReturnsNonNilCancel pins the noopObserver
// contract directly: even the noop must return a callable cancel
// (orchestrator code calls cancel() unconditionally per ADR-0030).
func TestNoopObserver_StartReturnsNonNilCancel(t *testing.T) {
	t.Parallel()
	c := noopObserver{}.Start(context.Background(), "tdd", PhaseRequest{})
	if c == nil {
		t.Fatal("noopObserver.Start returned nil cancel")
	}
	c() // must not panic
	c() // must be idempotent (orchestrator may call twice on error paths)
}
