package core

import (
	"context"
	"errors"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

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

func TestDispatchRunnerWithRetry_OptionalInfraSkipParity(t *testing.T) {
	runner := &alwaysFailRunner{name: "evaluator", err: ErrArtifactTimeout}
	o := retryParityOrchestrator(t, runner, "evaluator", phasespec.PhaseSpec{Optional: true})
	cr := retryParityCycleRun(o, t)

	resp, attempts, err := cr.dispatchRunnerWithRetry(Phase("evaluator"), PhaseRequest{})

	if err != nil {
		t.Fatalf("optional off-floor phase exhausting infra retries must degrade to SKIPPED with warning + advance (err==nil), got err=%v", err)
	}
	if resp.Verdict != VerdictSKIPPED {
		t.Errorf("degraded response verdict = %q, want %q", resp.Verdict, VerdictSKIPPED)
	}
	if attempts != o.retryConfig.PhaseMaxAttempts {
		t.Errorf("attempts = %d, want %d (retries must still exhaust before degrading)", attempts, o.retryConfig.PhaseMaxAttempts)
	}
	if runner.n != o.retryConfig.PhaseMaxAttempts {
		t.Errorf("runner invoked %d times, want %d — the skip must not short-circuit the retry loop itself", runner.n, o.retryConfig.PhaseMaxAttempts)
	}
}

func TestDispatchRunnerWithRetry_PostShipObserverSkipParity(t *testing.T) {
	runner := &alwaysFailRunner{name: "memo", err: errors.New("memo tier/envelope policy error")}
	o := retryParityOrchestrator(t, runner, "memo", phasespec.PhaseSpec{Optional: true, After: "ship"})
	cr := retryParityCycleRun(o, t)
	cr.cs.Shipped = true // ship already recorded PASS this cycle

	resp, _, err := cr.dispatchRunnerWithRetry(Phase("memo"), PhaseRequest{})

	if err != nil {
		t.Fatalf("post-ship best-effort observer failure on an already-shipped cycle must degrade to SKIPPED with warning + advance (err==nil), got err=%v", err)
	}
	if resp.Verdict != VerdictSKIPPED {
		t.Errorf("degraded response verdict = %q, want %q", resp.Verdict, VerdictSKIPPED)
	}
}

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
	if attempts != 1 {
		t.Errorf("attempts = %d, want 1 (non-infra errors are not retried)", attempts)
	}
	if runner.n != 1 {
		t.Errorf("runner invoked %d times, want 1", runner.n)
	}
}
