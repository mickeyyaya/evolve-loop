package main

// cmd_loop_wave_minwidth_test.go — RED contract for cycle-547's
// fleet-min-width-lane-fallback task.
//
// PROBLEM (scout Key Finding 2): cmd_loop.go's batch loop dispatches a wave
// via dispatchIteration; when that reports ran=false with a nil error (0
// lanes planned — either an empty triage plan, D1, or the wave's Count was
// quota/budget-shrunk to <=1 so shouldRunWave's Count>1 gate rejects it
// before planning), the loop unconditionally WARNs and falls through to the
// legacy sequential orch.RunCycle path — cmd_loop_wave.go's own doc calls
// this "the ONLY path that can leak into the main tree" (unisolated, runs in
// the process cwd instead of a dedicated worktree). A fleet.count=2 operator
// whose wave shrank to 1 lane via a quota bench gets width ZERO (sequential),
// not width 1 — defeating fleet.count in the worst way.
//
// FIX CONTRACT (new surface this cycle — undefined until Builder adds it, so
// this package's test build fails to compile today; that compile failure IS
// the RED evidence, mirroring the cycle-465/507 precedent):
//
//	forceOneLaneDispatch(ctx, preflight, planFn, launcher, waveIndex) —
//	drives up to ONE disjoint candidate through the SAME isolated-worktree
//	path dispatchIteration uses (preflight -> planFn -> fleet.PlanFromTriage
//	capped at count=1 -> launcher.Run), WITHOUT the shouldRunWave(Count>1)
//	gate (the caller already knows the original fleet config wanted >1 lanes
//	and only reached here because the wave-sized Count shrank to <=1 — this
//	is the shrink-repair path, not the general multi-lane entry point).
//	Mirrors dispatchIteration's other safety contracts exactly: a preflight
//	refusal surfaces an error with planFn/launcher never invoked, and a
//	genuinely empty candidate backlog (PlanFromTriage adapts to zero specs)
//	reports ran=false, err=nil so the caller correctly falls back to
//	sequential — true sequential fallback stays reserved for that case.
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Positive : TestForceOneLaneDispatch_DispatchesIsolatedWaveWhenCandidateExists
//   - Negative : TestForceOneLaneDispatch_EmptyBacklogStaysFalseNoLauncherInvoked
//     (the strongest anti-no-op: a naive "always dispatch" impl fails here)
//   - Safety   : TestForceOneLaneDispatch_PreflightRefusalNeverPlansNorLaunches
//     (the S3 dirty-control-plane guard must still gate the repair path)
//   - Regression (guards against the WRONG fix): TestShouldRunWave_CountOneOrZeroStillFalse
//     pins that shouldRunWave itself is NOT loosened to Count>=1 — that would
//     also route an operator's genuinely-static fleet.count=1 config through
//     the wave path, violating "fleet.count=1 legacy path untouched".
import (
	"context"
	"errors"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// minWidthFakeLauncher records every Run call's specs so tests can assert
// exactly one isolated lane was dispatched (or none, for the empty-backlog
// case) — a locally-scoped fake (distinct name from fakeWaveLauncher in
// cmd_loop_wave_test.go) to avoid touching existing test files.
type minWidthFakeLauncher struct {
	calls [][]fleet.CycleSpec
}

func (f *minWidthFakeLauncher) Run(_ context.Context, specs []fleet.CycleSpec) []fleet.Result {
	f.calls = append(f.calls, specs)
	results := make([]fleet.Result, len(specs))
	for i := range specs {
		results[i] = fleet.Result{Index: i, ExitCode: 0}
	}
	return results
}

func TestForceOneLaneDispatch_DispatchesIsolatedWaveWhenCandidateExists(t *testing.T) {
	launcher := &minWidthFakeLauncher{}
	planFn := func(context.Context, int) ([]byte, []string, error) {
		return []byte(`{"committed_floors":["core"]}`), nil, nil
	}
	ran, specs, results, err := forceOneLaneDispatch(context.Background(), func() error { return nil }, planFn, launcher, 0)
	if err != nil {
		t.Fatalf("forceOneLaneDispatch returned error: %v, want nil", err)
	}
	if !ran {
		t.Fatalf("forceOneLaneDispatch did not dispatch a candidate as an isolated 1-lane wave (ran=false) — must not fall back to sequential when >=1 candidate exists")
	}
	if len(specs) != 1 {
		t.Fatalf("len(specs) = %d, want 1 (capped to a single lane)", len(specs))
	}
	if len(launcher.calls) != 1 {
		t.Fatalf("launcher.Run invoked %d times, want 1 — the candidate must go through the SAME isolated-worktree launcher path, not the process-cwd sequential path", len(launcher.calls))
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
}

func TestForceOneLaneDispatch_EmptyBacklogStaysFalseNoLauncherInvoked(t *testing.T) {
	launcher := &minWidthFakeLauncher{}
	planFn := func(context.Context, int) ([]byte, []string, error) {
		return []byte(`{"committed_floors":[]}`), nil, nil
	}
	ran, specs, _, err := forceOneLaneDispatch(context.Background(), func() error { return nil }, planFn, launcher, 0)
	if err != nil {
		t.Fatalf("forceOneLaneDispatch returned error: %v, want nil (a genuinely empty backlog is not an error)", err)
	}
	if ran {
		t.Fatalf("forceOneLaneDispatch reported ran=true with a genuinely empty triage plan (%v specs) — a naive 'always dispatch' impl must fail this; empty backlog must still fall back to sequential", specs)
	}
	if len(launcher.calls) != 0 {
		t.Fatalf("launcher.Run invoked %d times for an empty adapted plan, want 0", len(launcher.calls))
	}
}

func TestForceOneLaneDispatch_PreflightRefusalNeverPlansNorLaunches(t *testing.T) {
	launcher := &minWidthFakeLauncher{}
	refusal := errors.New("dirty control plane")
	planFnCalled := false
	planFn := func(context.Context, int) ([]byte, []string, error) {
		planFnCalled = true
		return []byte(`{"committed_floors":["core"]}`), nil, nil
	}
	ran, _, _, err := forceOneLaneDispatch(context.Background(), func() error { return refusal }, planFn, launcher, 0)
	if err == nil {
		t.Fatalf("forceOneLaneDispatch swallowed a preflight refusal — want a surfaced error")
	}
	if ran {
		t.Fatalf("forceOneLaneDispatch reported ran=true despite a preflight refusal")
	}
	if planFnCalled {
		t.Errorf("planFn was invoked despite a preflight refusal — the S3 dirty-control-plane guard must gate BEFORE planning, same as dispatchIteration")
	}
	if len(launcher.calls) != 0 {
		t.Errorf("launcher invoked despite a preflight refusal")
	}
}

// TestShouldRunWave_CountOneOrZeroStillFalse guards against the wrong fix: the
// min-width repair must be a NEW seam (forceOneLaneDispatch) invoked
// specifically when a wave-sized Count shrank below the gate, not a loosened
// shouldRunWave(Count>=1) — the latter would ALSO route an operator's
// genuinely-static fleet.count=1 config through the wave path at cmd_loop.go's
// outer `if shouldRunWave(fleetCfg)` gate, violating "fleet.count=1 legacy
// path untouched".
func TestShouldRunWave_CountOneOrZeroStillFalse(t *testing.T) {
	if shouldRunWave(policy.FleetConfig{Count: 1, PlanSource: "triage"}) {
		t.Fatalf("shouldRunWave(Count:1, triage) = true, want false — Count=1 must keep the existing sequential orch.RunCycle path untouched")
	}
	if shouldRunWave(policy.FleetConfig{Count: 0, PlanSource: "triage"}) {
		t.Fatalf("shouldRunWave(Count:0, triage) = true, want false")
	}
}
