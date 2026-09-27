package main

import (
	"context"
	"errors"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type waveCtxKey string

func waveEligibleFC() policy.FleetConfig {
	return policy.FleetConfig{Count: 2, Concurrency: 2, PlanSource: "triage"}
}

func passingPreflight() error { return nil }

// TestDispatchIteration_CtxReachesPlanFn threads a context value through ctx
// so a re-minted context.Background() (which could never carry it) is
// distinguishable from the caller's actual ctx reaching the plan function.
func TestDispatchIteration_CtxReachesPlanFn(t *testing.T) {
	const key = waveCtxKey("wave-ctx-probe")
	ctx := context.WithValue(context.Background(), key, "threaded")
	launcher := &fakeWaveLauncher{}
	var seen any
	planFn := func(ctx context.Context, waveIndex int) ([]byte, []string, error) {
		seen = ctx.Value(key)
		return []byte(`{"committed_floors":["bridge","core"]}`), nil, nil
	}
	ran, _, _, err := dispatchIteration(ctx, waveEligibleFC(), passingPreflight, planFn, launcher, nil, 0)
	if err != nil {
		t.Fatalf("dispatchIteration: %v", err)
	}
	if !ran {
		t.Fatalf("dispatchIteration did not take the wave path for fleet{count:2, plan_source:triage}")
	}
	if seen != "threaded" {
		t.Errorf("planFn ctx.Value = %v, want %q — the caller's ctx must thread into the plan path, never a re-minted context.Background()", seen, "threaded")
	}
}

func TestDispatchIteration_CancelledCtxObservableInPlanFn(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	launcher := &fakeWaveLauncher{}
	planFn := func(ctx context.Context, waveIndex int) ([]byte, []string, error) {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		return nil, nil, errors.New("plan ctx was not cancelled — cancellation did not propagate")
	}
	ran, _, _, err := dispatchIteration(ctx, waveEligibleFC(), passingPreflight, planFn, launcher, nil, 0)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("dispatchIteration error = %v, want it to wrap context.Canceled from the plan path", err)
	}
	if ran || len(launcher.calls) != 0 {
		t.Errorf("cancelled ctx: ran=%v launcher.calls=%d, want ran=false and zero launches", ran, len(launcher.calls))
	}
}

func TestDispatchIteration_PreflightRefusalNeverPlansNorLaunches(t *testing.T) {
	refusal := errors.New(`fleet: control-plane file ".evolve/policy.json" has uncommitted changes; commit it via evolve ship --class manual before dispatching a wave`)
	launcher := &fakeWaveLauncher{}
	planFn := func(ctx context.Context, waveIndex int) ([]byte, []string, error) {
		t.Fatal("planFn must never run when the control-plane preflight refuses the wave")
		return nil, nil, nil
	}
	ran, _, _, err := dispatchIteration(context.Background(), waveEligibleFC(), func() error { return refusal }, planFn, launcher, nil, 0)
	if !errors.Is(err, refusal) {
		t.Fatalf("dispatchIteration error = %v, want it to wrap the preflight refusal", err)
	}
	if ran {
		t.Errorf("dispatchIteration reported ran=true on a preflight refusal — must fall back to sequential")
	}
	if len(launcher.calls) != 0 {
		t.Errorf("launcher.Run invoked %d times on a preflight refusal, want 0", len(launcher.calls))
	}
}

func TestDispatchIteration_PreflightCleanWaveProceeds(t *testing.T) {
	launcher := &fakeWaveLauncher{}
	planFn := func(ctx context.Context, waveIndex int) ([]byte, []string, error) {
		return []byte(`{"committed_floors":["bridge","core"]}`), nil, nil
	}
	ran, specs, _, err := dispatchIteration(context.Background(), waveEligibleFC(), passingPreflight, planFn, launcher, nil, 0)
	if err != nil {
		t.Fatalf("dispatchIteration with clean preflight: %v", err)
	}
	if !ran || len(specs) == 0 {
		t.Fatalf("clean preflight: ran=%v len(specs)=%d, want a normally-dispatched wave", ran, len(specs))
	}
	if len(launcher.calls) != 1 {
		t.Fatalf("launcher.Run called %d times, want 1", len(launcher.calls))
	}
}

func TestDispatchIteration_SequentialPathNeverRunsPreflight(t *testing.T) {
	fc := policy.FleetConfig{Count: 1, Concurrency: 1, PlanSource: "triage"}
	launcher := &fakeWaveLauncher{}
	preflight := func() error {
		t.Fatal("preflight must never run on the sequential (Count==1) path")
		return nil
	}
	planFn := func(ctx context.Context, waveIndex int) ([]byte, []string, error) {
		t.Fatal("planFn must never run on the sequential (Count==1) path")
		return nil, nil, nil
	}
	ran, _, _, err := dispatchIteration(context.Background(), fc, preflight, planFn, launcher, nil, 0)
	if err != nil {
		t.Fatalf("dispatchIteration: %v", err)
	}
	if ran || len(launcher.calls) != 0 {
		t.Errorf("sequential path: ran=%v launcher.calls=%d, want ran=false and zero launches", ran, len(launcher.calls))
	}
}
