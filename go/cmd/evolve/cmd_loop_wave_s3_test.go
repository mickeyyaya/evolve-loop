package main

// cmd_loop_wave_s3_test.go — fleet-s3-guards AC1/AC2/AC5 (cycle 467):
// RED-first contract for the S3 wave guards at the dispatch seam. This file
// pins the POST-S3 signatures, so the cmd/evolve package fails to COMPILE
// until Builder lands them — that compile failure IS the RED evidence:
//
//	type wavePlanFn func(ctx context.Context, waveIndex int) ([]byte, []string, error)
//	dispatchIteration(ctx, fc, preflight func() error, planFn, launcher, nil, waveIndex)
//
// AC5 (reviewer note on PR #298): the loop's cancellable ctx must thread
// through the PLAN path too — productionWavePlanFn currently minted
// context.Background() at cmd_loop_wave.go:116, so cancellation never
// reached readLastCycleNumber. wavePlanFn gains the ctx parameter and the
// context.Background() mint dies (the paired ACS predicate asserts its
// absence from cmd_loop_wave.go).
//
// AC1/AC2 (wiring): the dirty-control-plane preflight gates the WAVE PATH
// ONLY, through an injected func() error — same injected-fn seam style as
// wavePlanFn/waveLauncher (production wiring: a closure over
// fleet.PreflightControlPlane(cfg.ProjectRoot)). A preflight refusal is
// surfaced as a wrapped error with ran=false so cmd_loop.go's existing WARN
// branch falls back to sequential; the launcher AND planFn are never invoked.
//
// Builder note: the pre-existing tests in cmd_loop_wave_test.go /
// cmd_loop_wave_amplify_test.go use the OLD signatures — update their call
// sites MECHANICALLY (add the ctx param to planFn literals, pass a nil-error
// preflight), preserving their names and assertions. Do NOT weaken or rename
// any test in THIS file.

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

// TestDispatchIteration_CtxReachesPlanFn (AC5): the exact ctx handed to
// dispatchIteration — not a fresh context.Background() — must reach the plan
// function. Pinned via a context value: a re-minted Background can never
// carry it.
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

// TestDispatchIteration_CancelledCtxObservableInPlanFn (AC5, negative): a
// plan function that honours cancellation must be ABLE to — its ctx arg
// reflects the caller's cancelled ctx, and its resulting error surfaces
// (wrapped, errors.Is-matchable) with ran=false and zero launches.
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

// TestDispatchIteration_PreflightRefusalNeverPlansNorLaunches (AC1,
// negative): a preflight refusal must surface its error (wrapped,
// errors.Is-matchable, actionable text intact) with ran=false, and NEITHER
// the plan function NOR the launcher may run — the guard fires before any
// wave work starts. Gaming fake it kills: a preflight consulted only after
// the lanes are already planned/launched.
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

// TestDispatchIteration_PreflightCleanWaveProceeds (AC2): a passing
// preflight must be invisible — the wave plans and launches exactly as
// before the guard existed (no false positives at the seam).
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

// TestDispatchIteration_SequentialPathNeverRunsPreflight (AC2, edge): with
// no fleet block (Count==1) the wave path is not taken and the preflight —
// which shells git against the main checkout in production — must NOT run.
// The guard gates waves, not the sequential loop.
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
