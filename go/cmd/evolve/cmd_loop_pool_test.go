package main

// cmd_loop_pool_test.go — RED contract for cycle-553's
// supervisor-continuous-lane-keeping task (the DISPATCHER-WIRING half of the
// L5 "ceiling-keeper"; the fleet-layer primitive fleet.RunPool already shipped
// cycle 550 with its own exhaustive pool_test.go).
//
// PROBLEM (triage-report.md sole `## top_n` item, inbox weight 0.95): cycle 550
// shipped fleet.RunPool (rolling lane pool that BACKFILLS a replacement lane the
// instant any lane exits) AND a policy knob that parses fleet.scheduling=="pool"
// into policy.FleetConfig.Scheduling (policy.go:1032) — but the loop DISPATCHER
// (cmd_loop.go's batch loop) still only ever consults shouldRunWave; it never
// reads fleetCfg.Scheduling, so a "pool"-scheduled operator STILL gets the wave
// barrier. RunPool has ZERO call sites outside its own package/test. The knob is
// wired to nothing.
//
// FIX CONTRACT (new surface this cycle — undefined until Builder adds it, so
// this package's test build fails to compile today; that compile failure IS the
// RED evidence, mirroring the cycle-465/507/547/550 precedent):
//
//	// poolPlanFn produces the pool's backlog of file-disjoint todos to roll
//	// through (the pool analogue of wavePlanFn's decisionJSON+cardPackages).
//	type poolPlanFn func(ctx context.Context, waveIndex int) ([]fleet.Todo, error)
//
//	// shouldRunPool gates the rolling-pool dispatch path. It requires the SAME
//	// fleet preconditions as shouldRunWave (Count>1 && PlanSource=="triage")
//	// PLUS the resolved Scheduling strategy being "pool". Mutually exclusive
//	// with shouldRunWave: a default/"wave" fleet never enters the pool, and a
//	// "pool" fleet never enters the wave barrier (no double-dispatch).
//	func shouldRunPool(fc policy.FleetConfig) bool
//
//	// dispatchPoolIteration runs one iteration's pool path when shouldRunPool
//	// gates it on, and reports ran=false (no side effects) otherwise so the
//	// caller falls through unchanged. On the pool path, in order: runs preflight
//	// (the SAME S3 dirty-control-plane guard dispatchIteration uses — a refusal
//	// surfaces wrapped/errors.Is-matchable with ran=false and NEITHER planFn NOR
//	// launch invoked); obtains the backlog via planFn (ctx threaded); and drives
//	// it through fleet.RunPool with the injected launch (the SAME isolated launch
//	// seam the wave path's Supervisor uses — per L4 no dispatch ever takes the
//	// unisolated in-process sequential path), sizing PoolConfig{Target:fc.Count,
//	// Concurrency:fc.Concurrency}. An empty backlog reports ran=false, err=nil so
//	// the caller falls back instead of consuming a --max-cycles iteration doing
//	// no work (the pool analogue of dispatchIteration's D1 empty-plan guard).
//	// Returns the backlog and the per-lane RunPool results.
//	func dispatchPoolIteration(ctx context.Context, fc policy.FleetConfig,
//		preflight func() error, planFn poolPlanFn, launch fleet.LaunchFn,
//		waveIndex int) (ran bool, backlog []fleet.Todo, results []fleet.Result, err error)
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Positive  : TestDispatchPoolIteration_BackfillsReplacementWhileSiblingStillRunning
//     (the core wiring proof — a dispatcher that wired the WAVE BARRIER instead
//     of RunPool, or that no-ops, fails this)
//   - Negative  : TestDispatchPoolIteration_EmptyBacklogStaysFalseNoLaunch
//     (strongest anti-no-op: a naive "always ran=true" dispatcher fails here)
//   - Safety    : TestDispatchPoolIteration_PreflightRefusalNeverPlansNorLaunches
//   - Regression: TestDispatchPoolIteration_WaveConfigInertNoLaunch pins that the
//     new seam is INERT for a default/"wave" fleet — no regression to the wave
//     path; TestShouldRunWaveAndPool_MutuallyExclusive pins the two gates never
//     both fire (no double dispatch).
import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// poolSpecID extracts the todo ID a pool launch was handed. fleet.RunPool builds
// each lane's CycleSpec via poolSpec, which sets Scope=[]string{td.ID}; reading
// Scope[0] lets a main-package test identify lanes without importing fleet-
// internal helpers (fleet's own pool_test.go uses the package-private scopeID).
func poolSpecID(spec fleet.CycleSpec) string {
	if len(spec.Scope) == 0 {
		return ""
	}
	return spec.Scope[0]
}

// TestShouldRunPool_GateTable (AC1, decision table): the pool path fires ONLY
// for a fleet (Count>1) with triage plan source AND the resolved Scheduling
// strategy "pool". A default/absent or "wave" Scheduling, a single-lane Count,
// or a manual plan source all keep it OFF — the new (unsoaked) mode is never
// entered by any config that did not explicitly opt in.
func TestShouldRunPool_GateTable(t *testing.T) {
	cases := []struct {
		name string
		fc   policy.FleetConfig
		want bool
	}{
		{"pool-scheduled fleet", policy.FleetConfig{Count: 2, PlanSource: "triage", Scheduling: "pool"}, true},
		{"wave-scheduled fleet", policy.FleetConfig{Count: 2, PlanSource: "triage", Scheduling: "wave"}, false},
		{"default scheduling absent", policy.FleetConfig{Count: 2, PlanSource: "triage", Scheduling: ""}, false},
		{"single lane cannot pool", policy.FleetConfig{Count: 1, PlanSource: "triage", Scheduling: "pool"}, false},
		{"manual plan source", policy.FleetConfig{Count: 2, PlanSource: "manual", Scheduling: "pool"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldRunPool(tc.fc); got != tc.want {
				t.Errorf("shouldRunPool(%+v) = %v, want %v", tc.fc, got, tc.want)
			}
		})
	}
}

// TestShouldRunWaveAndPool_MutuallyExclusive (AC2): the two dispatch gates must
// be disjoint over EVERY config — if both could ever fire for the same
// FleetConfig the batch loop could double-dispatch (a wave AND a pool for one
// iteration). Specifically a "pool" fleet must take pool-only (wave=false) and a
// default/"wave" fleet must take wave-only (pool=false), so wiring the pool
// branch cannot regress the shipped wave path.
func TestShouldRunWaveAndPool_MutuallyExclusive(t *testing.T) {
	configs := []policy.FleetConfig{
		{Count: 2, PlanSource: "triage", Scheduling: "pool"},
		{Count: 2, PlanSource: "triage", Scheduling: "wave"},
		{Count: 2, PlanSource: "triage", Scheduling: ""},
		{Count: 2, PlanSource: "manual", Scheduling: "pool"},
		{Count: 1, PlanSource: "triage", Scheduling: "pool"},
	}
	for _, fc := range configs {
		if shouldRunWave(fc) && shouldRunPool(fc) {
			t.Errorf("both gates fired for %+v — wave and pool dispatch must be mutually exclusive (double-dispatch)", fc)
		}
	}
	// Directional pins: pool config selects pool not wave; wave/default config
	// selects wave not pool.
	poolFC := policy.FleetConfig{Count: 2, PlanSource: "triage", Scheduling: "pool"}
	if shouldRunWave(poolFC) {
		t.Errorf("shouldRunWave(%+v) = true, want false — a pool-scheduled fleet must NOT take the wave barrier", poolFC)
	}
	waveFC := policy.FleetConfig{Count: 2, PlanSource: "triage", Scheduling: "wave"}
	if shouldRunPool(waveFC) {
		t.Errorf("shouldRunPool(%+v) = true, want false — a wave-scheduled fleet must NOT take the pool", waveFC)
	}
	defaultFC := policy.FleetConfig{Count: 2, PlanSource: "triage", Scheduling: ""}
	if !shouldRunWave(defaultFC) {
		t.Errorf("shouldRunWave(%+v) = false, want true — the default (absent scheduling) fleet must keep the shipped wave path (no regression)", defaultFC)
	}
}

// TestDispatchPoolIteration_BackfillsReplacementWhileSiblingStillRunning (AC3,
// positive — the core wiring proof): the dispatcher must route the backlog
// through fleet.RunPool, whose defining behavior over the wave barrier is
// backfilling a replacement lane the instant one lane exits while a sibling is
// STILL RUNNING. Three mutually file-disjoint todos, fc.Count=2: the pool fills
// with A and B; B returns immediately; C (the only remaining disjoint pending
// todo) MUST be dispatched before A — still blocked — is unblocked. A dispatcher
// that wired the WAVE BARRIER (Supervisor.Run: launch all, wait for ALL, then
// re-plan) instead of RunPool never observes C dispatched while A is in flight
// and fails here; so does a no-op that never launches.
func TestDispatchPoolIteration_BackfillsReplacementWhileSiblingStillRunning(t *testing.T) {
	fc := policy.FleetConfig{Count: 2, Concurrency: 2, PlanSource: "triage", Scheduling: "pool"}
	backlog := []fleet.Todo{
		{ID: "A", Files: []string{"a.go"}},
		{ID: "B", Files: []string{"b.go"}},
		{ID: "C", Files: []string{"c.go"}},
	}
	planFn := func(context.Context, int) ([]fleet.Todo, error) { return backlog, nil }

	holdA := make(chan struct{})
	dispatched := make(chan string, len(backlog))
	launch := func(_ context.Context, spec fleet.CycleSpec) (int, error) {
		id := poolSpecID(spec)
		dispatched <- id
		if id == "A" {
			<-holdA
		}
		return 0, nil
	}

	type outcome struct {
		ran     bool
		results []fleet.Result
		err     error
	}
	done := make(chan outcome, 1)
	go func() {
		ran, _, results, err := dispatchPoolIteration(context.Background(), fc, func() error { return nil }, planFn, launch, 0)
		done <- outcome{ran, results, err}
	}()

	// Initial fill must be exactly {A,B}.
	seen := map[string]bool{}
	for len(seen) < 2 {
		select {
		case id := <-dispatched:
			seen[id] = true
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for the initial 2-lane fill; got %v", seen)
		}
	}
	if !seen["A"] || !seen["B"] {
		t.Fatalf("initial fill dispatched %v, want exactly {A,B}", seen)
	}

	// B has already returned; A is still blocked. The replacement for B's exit —
	// C — must dispatch NOW, while A is still running, or the wave barrier was
	// never removed (the dispatcher did not wire RunPool).
	select {
	case id := <-dispatched:
		if id != "C" {
			t.Fatalf("backfill dispatched %q, want C (the only remaining disjoint pending todo)", id)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no replacement lane dispatched for B's exit while sibling A still ran — dispatchPoolIteration did not wire fleet.RunPool (wave barrier still in place)")
	}

	close(holdA)
	select {
	case o := <-done:
		if o.err != nil {
			t.Fatalf("dispatchPoolIteration returned error: %v, want nil", o.err)
		}
		if !o.ran {
			t.Fatalf("dispatchPoolIteration reported ran=false on a non-empty pool backlog")
		}
		if len(o.results) != 3 {
			t.Fatalf("len(results) = %d, want 3 (one per backlog item)", len(o.results))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("dispatchPoolIteration did not return after all 3 pool lanes finished")
	}
}

// TestDispatchPoolIteration_EmptyBacklogStaysFalseNoLaunch (AC4, negative — the
// strongest anti-no-op): a planFn that yields zero todos must report ran=false,
// err=nil and NEVER invoke launch, so the caller falls back instead of burning a
// --max-cycles iteration on an empty pool. A naive "always ran=true / always
// call RunPool" dispatcher fails here.
func TestDispatchPoolIteration_EmptyBacklogStaysFalseNoLaunch(t *testing.T) {
	fc := policy.FleetConfig{Count: 2, Concurrency: 2, PlanSource: "triage", Scheduling: "pool"}
	launched := 0
	launch := func(context.Context, fleet.CycleSpec) (int, error) { launched++; return 0, nil }
	planFn := func(context.Context, int) ([]fleet.Todo, error) { return nil, nil }

	ran, _, results, err := dispatchPoolIteration(context.Background(), fc, func() error { return nil }, planFn, launch, 0)
	if err != nil {
		t.Fatalf("dispatchPoolIteration returned error: %v, want nil (an empty backlog is not an error)", err)
	}
	if ran {
		t.Fatalf("dispatchPoolIteration reported ran=true on an empty pool backlog — must report ran=false so the caller falls back")
	}
	if launched != 0 {
		t.Fatalf("launch invoked %d times for an empty backlog, want 0", launched)
	}
	if len(results) != 0 {
		t.Fatalf("len(results) = %d, want 0 for an empty backlog", len(results))
	}
}

// TestDispatchPoolIteration_WaveConfigInertNoLaunch (AC2 regression): the new
// pool seam must be INERT for a fleet that did not opt into pool scheduling — a
// default/"wave" config takes the shouldRunPool=false branch (ran=false, launch
// never invoked), so introducing the pool branch cannot perturb the shipped wave
// path.
func TestDispatchPoolIteration_WaveConfigInertNoLaunch(t *testing.T) {
	fc := policy.FleetConfig{Count: 2, Concurrency: 2, PlanSource: "triage", Scheduling: "wave"}
	launched := 0
	launch := func(context.Context, fleet.CycleSpec) (int, error) { launched++; return 0, nil }
	planCalled := false
	planFn := func(context.Context, int) ([]fleet.Todo, error) {
		planCalled = true
		return []fleet.Todo{{ID: "A", Files: []string{"a.go"}}}, nil
	}

	ran, _, _, err := dispatchPoolIteration(context.Background(), fc, func() error { return nil }, planFn, launch, 0)
	if err != nil {
		t.Fatalf("dispatchPoolIteration returned error: %v, want nil on the gated-off path", err)
	}
	if ran {
		t.Fatalf("dispatchPoolIteration reported ran=true for a wave-scheduled fleet — the pool seam must be inert unless Scheduling==\"pool\"")
	}
	if planCalled {
		t.Errorf("planFn invoked for a wave-scheduled fleet — the gate must short-circuit BEFORE planning")
	}
	if launched != 0 {
		t.Errorf("launch invoked for a wave-scheduled fleet, want 0")
	}
}

// TestDispatchPoolIteration_PreflightRefusalNeverPlansNorLaunches (AC5, safety):
// the pool path must preserve dispatchIteration's S3 dirty-control-plane guard —
// a preflight refusal surfaces a wrapped (errors.Is-matchable) error with
// ran=false and NEITHER planFn NOR launch ever invoked.
func TestDispatchPoolIteration_PreflightRefusalNeverPlansNorLaunches(t *testing.T) {
	fc := policy.FleetConfig{Count: 2, Concurrency: 2, PlanSource: "triage", Scheduling: "pool"}
	refusal := errors.New("dirty control plane")
	planCalled, launched := false, 0
	planFn := func(context.Context, int) ([]fleet.Todo, error) {
		planCalled = true
		return []fleet.Todo{{ID: "A", Files: []string{"a.go"}}}, nil
	}
	launch := func(context.Context, fleet.CycleSpec) (int, error) { launched++; return 0, nil }

	ran, _, _, err := dispatchPoolIteration(context.Background(), fc, func() error { return refusal }, planFn, launch, 0)
	if err == nil {
		t.Fatalf("dispatchPoolIteration swallowed a preflight refusal — want a surfaced error")
	}
	if !errors.Is(err, refusal) {
		t.Errorf("returned error does not wrap the preflight refusal (errors.Is=false): %v", err)
	}
	if ran {
		t.Fatalf("dispatchPoolIteration reported ran=true despite a preflight refusal")
	}
	if planCalled {
		t.Errorf("planFn invoked despite a preflight refusal — the guard must gate BEFORE planning")
	}
	if launched != 0 {
		t.Errorf("launch invoked despite a preflight refusal")
	}
}
