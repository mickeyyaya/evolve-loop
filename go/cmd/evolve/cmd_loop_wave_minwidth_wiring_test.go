package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// wiringFakeLauncher records every Run call's specs — distinct name from the
// other fake launchers in this package's test files (each scoped to its own
// file to avoid cross-file coupling), per the minWidthFakeLauncher precedent.
type wiringFakeLauncher struct {
	calls [][]fleet.CycleSpec
}

func (f *wiringFakeLauncher) Run(_ context.Context, specs []fleet.CycleSpec) []fleet.Result {
	f.calls = append(f.calls, specs)
	results := make([]fleet.Result, len(specs))
	for i := range specs {
		results[i] = fleet.Result{Index: i, ExitCode: 0}
	}
	return results
}

func TestMinWidthRepair_GuardNotMetNeverInvokesLauncher(t *testing.T) {
	launcher := &wiringFakeLauncher{}
	preflightCalled, planFnCalled := false, false
	preflight := func() error { preflightCalled = true; return nil }
	planFn := func(context.Context, int) ([]byte, []string, error) {
		planFnCalled = true
		return []byte(`{"committed_floors":["core"]}`), nil, nil
	}
	var stderr bytes.Buffer

	handled := minWidthRepair(context.Background(),
		policy.FleetConfig{Count: 1}, policy.FleetConfig{Count: 1},
		preflight, planFn, launcher, nil, 0, &stderr, testRootSignals(t, &stderr))

	if handled {
		t.Fatal("minWidthRepair must report handled=false when fleetCfg.Count<=1 — the repair is not eligible at all")
	}
	if preflightCalled || planFnCalled {
		t.Error("an ineligible guard must never invoke preflight or planFn")
	}
	if len(launcher.calls) != 0 {
		t.Errorf("an ineligible guard must never invoke the launcher; got %d calls", len(launcher.calls))
	}
	if !strings.Contains(stderr.String(), "empty triage plan") {
		t.Errorf("ineligible guard must WARN the empty-triage-plan message; got %q", stderr.String())
	}
}

func TestMinWidthRepair_GuardMetDispatchesOneIsolatedLaneAndSignalsContinue(t *testing.T) {
	launcher := &wiringFakeLauncher{}
	planFn := func(context.Context, int) ([]byte, []string, error) {
		return []byte(`{"committed_floors":["core"]}`), nil, nil
	}
	var stderr bytes.Buffer

	handled := minWidthRepair(context.Background(),
		policy.FleetConfig{Count: 3}, policy.FleetConfig{Count: 1},
		func() error { return nil }, planFn, launcher, nil, 2, &stderr, testRootSignals(t, &stderr))

	if !handled {
		t.Fatal("minWidthRepair must report handled=true when a candidate is dispatched — the caller must continue, not fall through to sequential")
	}
	if len(launcher.calls) != 1 {
		t.Fatalf("launcher.Run invoked %d times, want exactly 1 isolated lane", len(launcher.calls))
	}
	if len(launcher.calls[0]) != 1 {
		t.Errorf("dispatched %d specs, want 1 (capped to a single lane)", len(launcher.calls[0]))
	}
	if !strings.Contains(stderr.String(), "min-width repair dispatched") {
		t.Errorf("a successful repair must log the min-width-repair-dispatched message; got %q", stderr.String())
	}
}

func TestMinWidthRepair_EligibleButEmptyBacklogFallsBackToSequential(t *testing.T) {
	launcher := &wiringFakeLauncher{}
	planFn := func(context.Context, int) ([]byte, []string, error) {
		return []byte(`{"committed_floors":[]}`), nil, nil
	}
	var stderr bytes.Buffer

	handled := minWidthRepair(context.Background(),
		policy.FleetConfig{Count: 2}, policy.FleetConfig{Count: 1},
		func() error { return nil }, planFn, launcher, nil, 0, &stderr, testRootSignals(t, &stderr))

	if handled {
		t.Fatal("an empty adapted backlog must report handled=false — true sequential fallback")
	}
	if len(launcher.calls) != 0 {
		t.Errorf("launcher invoked %d times for an empty adapted plan, want 0", len(launcher.calls))
	}
	if !strings.Contains(stderr.String(), "empty backlog") {
		t.Errorf("empty-backlog edge must WARN the empty-backlog message; got %q", stderr.String())
	}
}

func TestMinWidthRepair_ForceDispatchErrorSurfacesAndFallsBack(t *testing.T) {
	launcher := &wiringFakeLauncher{}
	refusal := errors.New("dirty control plane")
	planFn := func(context.Context, int) ([]byte, []string, error) {
		return []byte(`{"committed_floors":["core"]}`), nil, nil
	}
	var stderr bytes.Buffer

	handled := minWidthRepair(context.Background(),
		policy.FleetConfig{Count: 2}, policy.FleetConfig{Count: 1},
		func() error { return refusal }, planFn, launcher, nil, 0, &stderr, testRootSignals(t, &stderr))

	if handled {
		t.Fatal("a preflight refusal must report handled=false")
	}
	if len(launcher.calls) != 0 {
		t.Errorf("launcher invoked despite a preflight refusal")
	}
	if !strings.Contains(stderr.String(), "min-width repair failed") || !strings.Contains(stderr.String(), refusal.Error()) {
		t.Errorf("the error must be surfaced in the WARN message, not swallowed; got %q", stderr.String())
	}
}
