package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// fakeWaveLauncher records every Run call's specs (input order = wave order)
// so tests can assert lane count / scope disjointness AND prove the golden
// no-block path never invokes it — the same fake-launcher pattern
// cmd_fleet.go's production execCycleLaunch is swapped out for.
type fakeWaveLauncher struct {
	calls [][]fleet.CycleSpec
}

func (f *fakeWaveLauncher) Run(_ context.Context, specs []fleet.CycleSpec) []fleet.Result {
	f.calls = append(f.calls, specs)
	results := make([]fleet.Result, len(specs))
	for i := range specs {
		results[i] = fleet.Result{Index: i, ExitCode: 0}
	}
	return results
}

func waveScopeIDs(spec fleet.CycleSpec) map[string]bool {
	ids := map[string]bool{}
	for _, id := range strings.Split(spec.Env[ipcenv.FleetScopeKey], ",") {
		if id != "" {
			ids[id] = true
		}
	}
	return ids
}

func TestShouldRunWave_GateTable(t *testing.T) {
	cases := []struct {
		name string
		fc   policy.FleetConfig
		want bool
	}{
		{"absent-block-default", policy.FleetConfig{Count: 1, Concurrency: 1, PlanSource: "triage"}, false},
		{"count-one-explicit-stays-sequential", policy.FleetConfig{Count: 1, Concurrency: 1, PlanSource: "triage"}, false},
		{"count-two-triage-runs-wave", policy.FleetConfig{Count: 2, Concurrency: 2, PlanSource: "triage"}, true},
		{"count-two-manual-plansource-stays-sequential", policy.FleetConfig{Count: 2, Concurrency: 2, PlanSource: "manual"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldRunWave(tc.fc); got != tc.want {
				t.Errorf("shouldRunWave(%+v) = %v, want %v", tc.fc, got, tc.want)
			}
		})
	}
}

func TestDispatchIteration_TwoWavesDisjointLaneScopes(t *testing.T) {
	fc := policy.FleetConfig{Count: 2, Concurrency: 2, PlanSource: "triage"}
	launcher := &fakeWaveLauncher{}
	waveFloors := [][]string{
		{"bridge", "core", "audit"},
		{"policy", "fleet", "ipcenv"},
	}
	planFn := func(_ context.Context, waveIndex int) ([]byte, []string, error) {
		floors := waveFloors[waveIndex]
		return []byte(`{"committed_floors":["` + strings.Join(floors, `","`) + `"]}`), nil, nil
	}

	seenAcrossWaves := map[string]bool{}
	for wave := 0; wave < 2; wave++ {
		ran, specs, _, err := dispatchIteration(context.Background(), fc, passingPreflight, planFn, launcher, nil, wave)
		if err != nil {
			t.Fatalf("wave %d: dispatchIteration returned error: %v", wave, err)
		}
		if !ran {
			t.Fatalf("wave %d: dispatchIteration did not take the wave path for fleet{count:2, plan_source:triage}", wave)
		}
		if len(specs) != 2 {
			t.Fatalf("wave %d: len(specs) = %d, want 2 (3 disjoint floors spread across 2 lanes)", wave, len(specs))
		}
		for _, spec := range specs {
			for id := range waveScopeIDs(spec) {
				if seenAcrossWaves[id] {
					t.Errorf("wave %d: todo id %q reused across waves/lanes — every lane's scope must be pairwise disjoint", wave, id)
				}
				seenAcrossWaves[id] = true
			}
		}
	}
	if len(launcher.calls) != 2 {
		t.Fatalf("launcher.Run called %d times, want 2 (--max-cycles 2 with count=2 => 2 waves)", len(launcher.calls))
	}
}

func TestDispatchIteration_AbsentFleetBlockStaysSequentialGolden(t *testing.T) {
	fc := policy.FleetConfig{Count: 1, Concurrency: 1, PlanSource: "triage"}
	launcher := &fakeWaveLauncher{}
	planFn := func(context.Context, int) ([]byte, []string, error) {
		t.Fatal("planFn must never be called when the wave path is not taken (Count==1)")
		return nil, nil, nil
	}
	ran, specs, results, err := dispatchIteration(context.Background(), fc, passingPreflight, planFn, launcher, nil, 0)
	if err != nil {
		t.Fatalf("dispatchIteration returned error: %v", err)
	}
	if ran {
		t.Fatalf("dispatchIteration ran the wave path for an absent fleet block (Count=1) — must take the existing sequential path instead")
	}
	if specs != nil || results != nil {
		t.Errorf("dispatchIteration returned non-nil specs/results (%v/%v) on the sequential path — no Supervisor may be constructed", specs, results)
	}
	if len(launcher.calls) != 0 {
		t.Fatalf("launcher.Run invoked %d times for an absent fleet block, want 0 (golden regression: no Supervisor construction on the sequential path)", len(launcher.calls))
	}
}

func TestDispatchIteration_MalformedTriagePlanFallsBackSequential(t *testing.T) {
	fc := policy.FleetConfig{Count: 2, Concurrency: 2, PlanSource: "triage"}
	launcher := &fakeWaveLauncher{}
	planFn := func(context.Context, int) ([]byte, []string, error) {
		return []byte(`{"committed_floors":[`), nil, nil // truncated JSON
	}
	ran, _, _, err := dispatchIteration(context.Background(), fc, passingPreflight, planFn, launcher, nil, 0)
	if err == nil {
		t.Fatalf("dispatchIteration(malformed triage plan) returned nil error — want an explicit error so the caller WARNs and falls back to sequential")
	}
	if ran {
		t.Errorf("dispatchIteration(malformed triage plan) reported ran=true — a parse failure must fall back to sequential, never guess a launch")
	}
	if len(launcher.calls) != 0 {
		t.Errorf("launcher.Run invoked %d times on a malformed triage plan, want 0", len(launcher.calls))
	}
}

func TestDispatchIteration_PlanFnErrorFallsBackSequential(t *testing.T) {
	fc := policy.FleetConfig{Count: 2, Concurrency: 2, PlanSource: "triage"}
	launcher := &fakeWaveLauncher{}
	wantErr := errors.New("triage phase failed")
	planFn := func(context.Context, int) ([]byte, []string, error) { return nil, nil, wantErr }
	ran, _, _, err := dispatchIteration(context.Background(), fc, passingPreflight, planFn, launcher, nil, 0)
	if !errors.Is(err, wantErr) {
		t.Fatalf("dispatchIteration error = %v, want it to wrap %v", err, wantErr)
	}
	if ran || len(launcher.calls) != 0 {
		t.Errorf("dispatchIteration(planFn error): ran=%v launcher.calls=%d, want ran=false and zero launches", ran, len(launcher.calls))
	}
}
