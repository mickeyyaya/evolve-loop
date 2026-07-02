package main

// cmd_loop_wave_test.go — RED-first contract for the loop-wave-dispatch seam
// (FLEET-AS-POLICY S2, salvaged from cycle 465's preserved worktree per
// cycle-466's operator T1: fix D1 empty-plan livelock). Pins the
// per-iteration seam cmd_loop.go's batch for-loop must call, factored into
// cmd_loop_wave.go: shouldRunWave (the Count>1 && PlanSource=="triage" gate,
// mirroring the consecutiveFailBreaker pure-decision-function precedent in
// cmd_loop_failbreaker_test.go) and dispatchIteration (obtains one wave's
// triage plan via the injected wavePlanFn, adapts it through
// fleet.PlanFromTriage, and launches through the injected waveLauncher —
// production wiring is *fleet.Supervisor + execCycleLaunch). None of these
// symbols exist yet; every test below fails to COMPILE until Builder adds
// cmd_loop_wave.go — that compile failure IS the RED evidence (mirrors
// cycle-465's precedent). Functions directly exercising dispatchIteration
// are named TestDispatchIteration_* (renamed from cycle 465's TestLoopWave_*
// prefix) so `go test -run 'TestDispatchIteration'` — the eval's AC1 grading
// command — exercises the full contract, including the D1 empty-plan guard
// added in cmd_loop_wave_amplify_test.go. See
// .evolve/evals/s2-wave-salvage-fix-d1.md for the acceptance criteria.

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

// TestShouldRunWave_GateTable (AC4, decision table): the wave path fires
// ONLY when Count>1 AND the resolved PlanSource is "triage" — an absent/
// Count==1 block, or a Count>1 block whose plan_source fell back to the
// closed-vocab "manual" default, must both keep the existing sequential
// orch.RunCycle body untouched.
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

// TestDispatchIteration_TwoWavesDisjointLaneScopes (AC1, positive): fleet{count:2,
// plan_source:triage} driven for 2 iterations (the --max-cycles 2 contract:
// each iteration IS a wave) must run the wave path both times, launch through
// the injected launcher exactly twice, and never repeat a scoped todo id
// across any lane in either wave. Gaming fake it kills: a wave loop that
// launches Count unscoped identical cycles.
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
		ran, specs, _, err := dispatchIteration(context.Background(), fc, passingPreflight, planFn, launcher, wave)
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

// TestDispatchIteration_AbsentFleetBlockStaysSequentialGolden (AC4,
// negative/golden): with no fleet block (Count==1, the FleetConfig()
// default), dispatchIteration must report ran=false and MUST NOT invoke the
// launcher or the plan function — the existing sequential orch.RunCycle path
// runs unchanged, no Supervisor is ever constructed. This test MUST fail if
// the wave path is unconditionally enabled.
func TestDispatchIteration_AbsentFleetBlockStaysSequentialGolden(t *testing.T) {
	fc := policy.FleetConfig{Count: 1, Concurrency: 1, PlanSource: "triage"}
	launcher := &fakeWaveLauncher{}
	planFn := func(context.Context, int) ([]byte, []string, error) {
		t.Fatal("planFn must never be called when the wave path is not taken (Count==1)")
		return nil, nil, nil
	}
	ran, specs, results, err := dispatchIteration(context.Background(), fc, passingPreflight, planFn, launcher, 0)
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

// TestDispatchIteration_MalformedTriagePlanFallsBackSequential (AC3,
// negative, beyond the eval's minimum bar): a malformed triage-decision.json
// for a wave-eligible config must surface a non-nil error and fall back to
// sequential — never a silent unscoped launch — mirroring PlanFromTriage's
// own fail-safe contract at the loop-integration layer.
func TestDispatchIteration_MalformedTriagePlanFallsBackSequential(t *testing.T) {
	fc := policy.FleetConfig{Count: 2, Concurrency: 2, PlanSource: "triage"}
	launcher := &fakeWaveLauncher{}
	planFn := func(context.Context, int) ([]byte, []string, error) {
		return []byte(`{"committed_floors":[`), nil, nil // truncated JSON
	}
	ran, _, _, err := dispatchIteration(context.Background(), fc, passingPreflight, planFn, launcher, 0)
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

// TestDispatchIteration_PlanFnErrorFallsBackSequential (negative): when the
// single-writer triage step itself fails (e.g. the triage phase errored), the
// wave path must surface that error (wrapped, so errors.Is still matches) and
// never invoke the launcher.
func TestDispatchIteration_PlanFnErrorFallsBackSequential(t *testing.T) {
	fc := policy.FleetConfig{Count: 2, Concurrency: 2, PlanSource: "triage"}
	launcher := &fakeWaveLauncher{}
	wantErr := errors.New("triage phase failed")
	planFn := func(context.Context, int) ([]byte, []string, error) { return nil, nil, wantErr }
	ran, _, _, err := dispatchIteration(context.Background(), fc, passingPreflight, planFn, launcher, 0)
	if !errors.Is(err, wantErr) {
		t.Fatalf("dispatchIteration error = %v, want it to wrap %v", err, wantErr)
	}
	if ran || len(launcher.calls) != 0 {
		t.Errorf("dispatchIteration(planFn error): ran=%v launcher.calls=%d, want ran=false and zero launches", ran, len(launcher.calls))
	}
}
