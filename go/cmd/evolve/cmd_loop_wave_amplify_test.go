package main

// cmd_loop_wave_amplify_test.go — test-amplification (salvaged from cycle
// 465) for the loop-wave-dispatch seam. Black-box against the spec only:
// shouldRunWave is the pure Count>1 && PlanSource=="triage" gate (closed
// vocab, no normalization at this layer), and dispatchIteration is the
// single per-iteration wave-or-sequential decision point whose injected
// planFn / launcher seams these tests drive with hostile inputs. Reuses the
// scaffold's fakeWaveLauncher / waveScopeIDs helpers (same package).
//
// TestDispatchIteration_EmptyPlanNeverClaimsAWave is the cycle-466 D1
// regression: cycle 465's audit (audit-report.md, confidence 0.95)
// independently reproduced this exact defect via its predecessor
// TestLoopWave_EmptyTriagePlanNeverClaimsAWave — dispatchIteration
// (cmd_loop_wave.go:56-70 in the 465 worktree) did not guard
// len(specs)==0, so an empty adapted plan invoked launcher.Run with a
// zero-lane spec list and returned ran=true, silently consuming a
// --max-cycles iteration doing zero work (the livelock class named in the
// cycle-466 goal). Renamed to the TestDispatchIteration_ prefix so this
// cycle's eval AC1 grading command (`go test -run 'TestDispatchIteration'`)
// exercises it directly; assertions preserved verbatim from the proven
// cycle-465 audit evidence, plus an explicit launcher.calls-count check.

import (
	"context"
	"errors"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// resultEchoLauncher returns a canned result slice regardless of specs, so
// tests can prove dispatchIteration propagates lane outcomes verbatim.
type resultEchoLauncher struct {
	canned []fleet.Result
	calls  int
	gotCtx context.Context
}

func (l *resultEchoLauncher) Run(ctx context.Context, _ []fleet.CycleSpec) []fleet.Result {
	l.calls++
	l.gotCtx = ctx
	return l.canned
}

// TestShouldRunWave_AdversarialGateTable (edge/negative): the gate must stay
// shut for zero/negative counts (fail-safe even if a caller bypasses
// FleetConfig()'s clamp) and for any PlanSource not exactly "triage" — the
// vocab is closed and case/space-sensitive because FleetConfig() already
// normalized unknowns to "manual" upstream. The upper extreme fires: the gate
// itself imposes no hidden count cap.
func TestShouldRunWave_AdversarialGateTable(t *testing.T) {
	cases := []struct {
		name string
		fc   policy.FleetConfig
		want bool
	}{
		{"count-zero-stays-sequential", policy.FleetConfig{Count: 0, Concurrency: 0, PlanSource: "triage"}, false},
		{"count-negative-stays-sequential", policy.FleetConfig{Count: -5, Concurrency: 1, PlanSource: "triage"}, false},
		{"plansource-uppercase-is-not-triage", policy.FleetConfig{Count: 2, Concurrency: 2, PlanSource: "Triage"}, false},
		{"plansource-empty-stays-sequential", policy.FleetConfig{Count: 2, Concurrency: 2, PlanSource: ""}, false},
		{"plansource-trailing-space-is-not-triage", policy.FleetConfig{Count: 2, Concurrency: 2, PlanSource: "triage "}, false},
		{"huge-count-triage-runs-wave", policy.FleetConfig{Count: 1 << 20, Concurrency: 4, PlanSource: "triage"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldRunWave(tc.fc); got != tc.want {
				t.Errorf("shouldRunWave(%+v) = %v, want %v", tc.fc, got, tc.want)
			}
		})
	}
}

// TestDispatchIteration_EmptyPlanNeverClaimsAWave (AC1, D1 regression, edge/
// livelock guard): a wave-eligible config whose triage plan commits NOTHING
// (no floors, no cards) has zero lanes to launch. dispatchIteration must not
// report a successful ran=true wave with zero specs — that would silently
// consume --max-cycles iterations doing no work — and must never invoke the
// launcher with an empty spec list. Gaming fake this kills: a guard that
// still returns ran=true with an empty results slice (caller `continue`s,
// the wave iteration is still consumed).
func TestDispatchIteration_EmptyPlanNeverClaimsAWave(t *testing.T) {
	fc := policy.FleetConfig{Count: 2, Concurrency: 2, PlanSource: "triage"}
	launcher := &fakeWaveLauncher{}
	planFn := func(int) ([]byte, []string, error) {
		return []byte(`{"committed_floors":[]}`), nil, nil
	}
	ran, specs, _, err := dispatchIteration(context.Background(), fc, planFn, launcher, 0)
	if err != nil {
		t.Fatalf("dispatchIteration(empty plan) returned error: %v, want nil (an empty plan is not itself an error)", err)
	}
	for _, call := range launcher.calls {
		if len(call) == 0 {
			t.Errorf("launcher.Run invoked with an empty spec list — an empty plan must never launch a zero-lane wave")
		}
	}
	if ran && err == nil && len(specs) == 0 {
		t.Errorf("dispatchIteration reported ran=true with zero specs and nil error — an empty triage plan must fall back to sequential (or error), never claim a do-nothing wave")
	}
	if len(launcher.calls) != 0 {
		t.Fatalf("launcher.Run invoked %d times for an empty adapted plan, want 0 — the livelock class (a --max-cycles iteration consumed by zero lanes) is exactly what this guard must prevent", len(launcher.calls))
	}
}

// TestDispatchIteration_LaneResultsPropagateVerbatim (positive/negative
// mix): lane failures are DATA for the loop's failure accounting, not a
// dispatch error. The results the launcher returns — including non-zero
// exits and lane errors — must come back verbatim, in order, with ran=true
// and a nil dispatch error. Kills a dispatcher that swallows, filters, or
// reorders failed lanes.
func TestDispatchIteration_LaneResultsPropagateVerbatim(t *testing.T) {
	fc := policy.FleetConfig{Count: 2, Concurrency: 2, PlanSource: "triage"}
	laneErr := errors.New("lane 1 wedged")
	canned := []fleet.Result{
		{Index: 0, ExitCode: 0},
		{Index: 1, ExitCode: 81, Err: laneErr},
	}
	launcher := &resultEchoLauncher{canned: canned}
	planFn := func(int) ([]byte, []string, error) {
		return []byte(`{"committed_floors":["core","audit"]}`), nil, nil
	}
	ran, _, results, err := dispatchIteration(context.Background(), fc, planFn, launcher, 0)
	if err != nil {
		t.Fatalf("dispatchIteration returned error: %v (lane failures must be results, not a dispatch error)", err)
	}
	if !ran {
		t.Fatalf("dispatchIteration did not take the wave path")
	}
	if launcher.calls != 1 {
		t.Fatalf("launcher.Run called %d times, want 1", launcher.calls)
	}
	if len(results) != len(canned) {
		t.Fatalf("len(results) = %d, want %d (verbatim passthrough)", len(results), len(canned))
	}
	for i := range canned {
		if results[i].Index != canned[i].Index || results[i].ExitCode != canned[i].ExitCode || !errors.Is(results[i].Err, canned[i].Err) {
			t.Errorf("results[%d] = %+v, want %+v (verbatim, in order)", i, results[i], canned[i])
		}
	}
}

// TestDispatchIteration_WaveIndexReachesPlanFn (seam contract): the
// waveIndex handed to dispatchIteration must reach the plan function
// unchanged — production wavePlanFn keys the wave's triage artifact off it,
// so an off-by-one here silently replans a stale wave.
func TestDispatchIteration_WaveIndexReachesPlanFn(t *testing.T) {
	fc := policy.FleetConfig{Count: 2, Concurrency: 2, PlanSource: "triage"}
	for _, waveIndex := range []int{0, 7} {
		got := -1
		planFn := func(i int) ([]byte, []string, error) {
			got = i
			return []byte(`{"committed_floors":["core"]}`), nil, nil
		}
		if _, _, _, err := dispatchIteration(context.Background(), fc, planFn, &fakeWaveLauncher{}, waveIndex); err != nil {
			t.Fatalf("waveIndex %d: dispatchIteration returned error: %v", waveIndex, err)
		}
		if got != waveIndex {
			t.Errorf("planFn received waveIndex %d, want %d", got, waveIndex)
		}
	}
}

// TestDispatchIteration_ContextReachesLauncher (seam contract): the caller's
// context must flow through to the launcher — it carries the wave's
// cancellation and per-cycle timeout lineage. A dispatcher that substitutes
// context.Background() would orphan running lanes on loop shutdown.
func TestDispatchIteration_ContextReachesLauncher(t *testing.T) {
	type ctxKey struct{}
	fc := policy.FleetConfig{Count: 2, Concurrency: 2, PlanSource: "triage"}
	launcher := &resultEchoLauncher{}
	planFn := func(int) ([]byte, []string, error) {
		return []byte(`{"committed_floors":["core"]}`), nil, nil
	}
	ctx := context.WithValue(context.Background(), ctxKey{}, "wave-ctx-sentinel")
	if _, _, _, err := dispatchIteration(ctx, fc, planFn, launcher, 0); err != nil {
		t.Fatalf("dispatchIteration returned error: %v", err)
	}
	if launcher.calls != 1 {
		t.Fatalf("launcher.Run called %d times, want 1", launcher.calls)
	}
	if got, _ := launcher.gotCtx.Value(ctxKey{}).(string); got != "wave-ctx-sentinel" {
		t.Errorf("launcher received a context without the caller's value (got %q) — the wave must propagate the loop's context, not substitute its own root", got)
	}
}

// TestDispatchIteration_CardsFallbackFlowsThroughDispatch (positive): the
// adapter's card-package fallback must survive the dispatch layer — a plan
// with no floors but committed-card packages still launches a scoped,
// disjoint wave.
func TestDispatchIteration_CardsFallbackFlowsThroughDispatch(t *testing.T) {
	fc := policy.FleetConfig{Count: 2, Concurrency: 2, PlanSource: "triage"}
	launcher := &fakeWaveLauncher{}
	planFn := func(int) ([]byte, []string, error) {
		return []byte(`{}`), []string{"core", "audit"}, nil
	}
	ran, specs, _, err := dispatchIteration(context.Background(), fc, planFn, launcher, 0)
	if err != nil {
		t.Fatalf("dispatchIteration returned error: %v", err)
	}
	if !ran {
		t.Fatalf("dispatchIteration did not take the wave path for a cards-fallback plan")
	}
	if len(specs) != 2 {
		t.Fatalf("len(specs) = %d, want 2 (one lane per card package)", len(specs))
	}
	seen := map[string]bool{}
	for _, spec := range specs {
		for id := range waveScopeIDs(spec) {
			if seen[id] {
				t.Errorf("todo id %q appears in more than one lane — cards fallback must stay pairwise disjoint", id)
			}
			seen[id] = true
		}
	}
	if !seen["core"] || !seen["audit"] {
		t.Errorf("scoped ids = %v, want both card packages", seen)
	}
}

// TestDispatchIteration_DuplicateFloorsNeverCoScheduled (S3 pre-regression
// at the S2 layer): a hostile/buggy triage plan repeating a floor id must
// never yield two lanes owning the same scope — overlapping scopes
// co-scheduled is the exact collision class fleet partitioning exists to
// prevent.
func TestDispatchIteration_DuplicateFloorsNeverCoScheduled(t *testing.T) {
	fc := policy.FleetConfig{Count: 2, Concurrency: 2, PlanSource: "triage"}
	launcher := &fakeWaveLauncher{}
	planFn := func(int) ([]byte, []string, error) {
		return []byte(`{"committed_floors":["core","core","audit"]}`), nil, nil
	}
	ran, specs, _, err := dispatchIteration(context.Background(), fc, planFn, launcher, 0)
	if err != nil {
		t.Fatalf("dispatchIteration returned error: %v", err)
	}
	if !ran {
		t.Fatalf("dispatchIteration did not take the wave path")
	}
	seen := map[string]bool{}
	for _, spec := range specs {
		for id := range waveScopeIDs(spec) {
			if seen[id] {
				t.Errorf("todo id %q owned by two lanes in one wave — overlapping scopes must never co-schedule", id)
			}
			seen[id] = true
		}
	}
	if len(launcher.calls) != 1 {
		t.Fatalf("launcher.Run called %d times, want 1", len(launcher.calls))
	}
}
