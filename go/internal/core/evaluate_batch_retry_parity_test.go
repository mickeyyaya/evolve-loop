package core

// evaluate_batch_retry_parity_test.go — RED contract for the fable5 deep-scan
// finding evaluate-batch-retry-parity (inbox weight 0.82, cycle-618 scout).
//
// Context. The sequential dispatch loop (cyclerun_dispatch.go) applies TWO
// skip predicates before treating a phase's exhausted retries as a
// cycle-level failure: optionalInfraSkip (an Optional, non-mandatory,
// off-floor phase whose exhaustion is infra-shaped degrades to WARN+advance)
// and postShipObserverSkip (a best-effort post-ship Control observer's
// failure never turns an already-shipped cycle abnormal). dispatchRunnerWithRetry
// (evaluate_batch.go) — the SAME per-phase retry loop, reused for the
// parallel-evaluate batch — has NO calls to either predicate: it returns the
// raw error on exhaustion unconditionally. A batched Optional evaluate phase
// (or a post-ship Control observer that happened to land in a batch) that
// exhausts retries therefore aborts the WHOLE cycle in the batched path where
// the identical phase would have degraded to WARN+advance in the sequential
// path — the copy-adapted-control-flow class of defect already fixed once in
// this codebase (statefile-rmw-flock-single-source, cycle 617). This blocks
// the parallel-evaluate enforce flip (memory: phase_timing_evidence) because
// flipping today would silently narrow the fail-open surface for every
// batched phase.
//
// RED today: dispatchRunnerWithRetry ignores both skip predicates, so the
// assertions below (err==nil, verdict WARN) fail against the current
// unconditional-error return — a real behavioral RED, not a compile error.

import (
	"context"
	"errors"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// alwaysFailRunner is a PhaseRunner stub that returns the configured error on
// every call, so dispatchRunnerWithRetry always exhausts PhaseMaxAttempts.
type alwaysFailRunner struct {
	name string
	err  error
	n    int
}

func (r *alwaysFailRunner) Name() string { return r.name }
func (r *alwaysFailRunner) Run(_ context.Context, _ PhaseRequest) (PhaseResponse, error) {
	r.n++
	return PhaseResponse{}, r.err
}

// retryParityOrchestrator builds a minimal orchestrator with a catalog-Optional
// evaluate phase and a mandatory spine, mirroring postShipObserverOrchestrator's
// shape but scoped to this task's two skip predicates.
func retryParityOrchestrator(t *testing.T, runner PhaseRunner, phase string, specOverrides phasespec.PhaseSpec) *Orchestrator {
	t.Helper()
	specOverrides.Name = phase
	cat, err := phasespec.Catalog{}.Merge([]phasespec.PhaseSpec{specOverrides})
	if err != nil {
		t.Fatalf("setup: catalog merge: %v", err)
	}
	cfg := config.RoutingConfig{Mandatory: []string{"build", "audit", "ship"}}
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, map[Phase]PhaseRunner{Phase(phase): runner},
		WithCatalog(cat), WithRouting(cfg, nil))
	o.retryConfig.PhaseMaxAttempts = 2
	return o
}

func retryParityCycleRun(o *Orchestrator, t *testing.T) *cycleRun {
	return &cycleRun{
		o:           o,
		ctx:         context.Background(),
		cycle:       1,
		cs:          CycleState{WorkspacePath: t.TempDir(), RunID: "r"},
		req:         CycleRequest{ProjectRoot: t.TempDir()},
		envSnap:     map[string]string{},
		ctxSnap:     map[string]string{},
		retryConfig: o.retryConfig,
	}
}

// TestDispatchRunnerWithRetry_OptionalInfraSkipParity — AC-1 (the core fix):
// an Optional, off-floor, non-mandatory phase that exhausts retries on an
// ErrArtifactTimeout must degrade to WARN+advance (err==nil) instead of
// propagating the error, matching optionalInfraSkip's sequential-path
// behavior.
func TestDispatchRunnerWithRetry_OptionalInfraSkipParity(t *testing.T) {
	runner := &alwaysFailRunner{name: "evaluator", err: ErrArtifactTimeout}
	o := retryParityOrchestrator(t, runner, "evaluator", phasespec.PhaseSpec{Optional: true})
	cr := retryParityCycleRun(o, t)

	resp, attempts, err := cr.dispatchRunnerWithRetry(Phase("evaluator"), PhaseRequest{})

	if err != nil {
		t.Fatalf("optional off-floor phase exhausting infra retries must degrade to WARN+advance (err==nil), got err=%v", err)
	}
	if resp.Verdict != VerdictWARN {
		t.Errorf("degraded response verdict = %q, want %q", resp.Verdict, VerdictWARN)
	}
	if attempts != o.retryConfig.PhaseMaxAttempts {
		t.Errorf("attempts = %d, want %d (retries must still exhaust before degrading)", attempts, o.retryConfig.PhaseMaxAttempts)
	}
	if runner.n != o.retryConfig.PhaseMaxAttempts {
		t.Errorf("runner invoked %d times, want %d — the skip must not short-circuit the retry loop itself", runner.n, o.retryConfig.PhaseMaxAttempts)
	}
}

// TestDispatchRunnerWithRetry_PostShipObserverSkipParity — AC-2: a best-effort
// post-ship Control observer phase (memo) that exhausts retries with a
// NON-infra error, on an already-shipped cycle, must degrade to WARN+advance
// — matching postShipObserverSkip's sequential-path behavior. This is the
// half optionalInfraSkip alone cannot cover (postShipObserverSkip fires on
// ANY error shape once shipped==true, not just infra-shaped ones).
func TestDispatchRunnerWithRetry_PostShipObserverSkipParity(t *testing.T) {
	runner := &alwaysFailRunner{name: "memo", err: errors.New("memo tier/envelope policy error")}
	o := retryParityOrchestrator(t, runner, "memo", phasespec.PhaseSpec{Optional: true, After: "ship"})
	cr := retryParityCycleRun(o, t)
	cr.shipped = true // ship already recorded PASS this cycle

	resp, _, err := cr.dispatchRunnerWithRetry(Phase("memo"), PhaseRequest{})

	if err != nil {
		t.Fatalf("post-ship best-effort observer failure on an already-shipped cycle must degrade to WARN+advance (err==nil), got err=%v", err)
	}
	if resp.Verdict != VerdictWARN {
		t.Errorf("degraded response verdict = %q, want %q", resp.Verdict, VerdictWARN)
	}
}

// TestDispatchRunnerWithRetry_NonSkippableErrorStillFatal is the negative /
// anti-no-op twin: a phase that is NOT catalog-Optional (or whose error is
// neither infra-shaped nor post-ship) must still propagate its error on
// exhaustion. This is the discriminator that forbids a degenerate
// "always degrade" implementation from passing the two tests above.
func TestDispatchRunnerWithRetry_NonSkippableErrorStillFatal(t *testing.T) {
	wantErr := errors.New("boom")
	runner := &alwaysFailRunner{name: "build", err: wantErr}
	o := retryParityOrchestrator(t, runner, "build", phasespec.PhaseSpec{Optional: false})
	cr := retryParityCycleRun(o, t)

	_, attempts, err := cr.dispatchRunnerWithRetry(Phase("build"), PhaseRequest{})

	if err == nil {
		t.Fatal("a mandatory, non-skippable phase's exhausted error must still propagate — swallowing it would weaken the integrity floor")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("returned error = %v, want it to wrap %v", err, wantErr)
	}
	// A non-infra, non-transient error is not retried at all (existing,
	// unrelated retry-loop behavior) — it fails fast on the first attempt.
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1 (non-infra errors are not retried)", attempts)
	}
	if runner.n != 1 {
		t.Errorf("runner invoked %d times, want 1", runner.n)
	}
}
