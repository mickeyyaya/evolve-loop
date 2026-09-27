package main

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
	ran, specs, results, err := forceOneLaneDispatch(context.Background(), func() error { return nil }, planFn, launcher, nil, 0)
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
	ran, specs, _, err := forceOneLaneDispatch(context.Background(), func() error { return nil }, planFn, launcher, nil, 0)
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
	ran, _, _, err := forceOneLaneDispatch(context.Background(), func() error { return refusal }, planFn, launcher, nil, 0)
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

func TestShouldRunWave_CountOneOrZeroStillFalse(t *testing.T) {
	if shouldRunWave(policy.FleetConfig{Count: 1, PlanSource: "triage"}) {
		t.Fatalf("shouldRunWave(Count:1, triage) = true, want false — Count=1 must keep the existing sequential orch.RunCycle path untouched")
	}
	if shouldRunWave(policy.FleetConfig{Count: 0, PlanSource: "triage"}) {
		t.Fatalf("shouldRunWave(Count:0, triage) = true, want false")
	}
}
