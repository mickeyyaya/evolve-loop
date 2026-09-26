package main

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// poolSpecID reads the todo ID from Scope[0], letting a main-package test
// identify lanes without importing fleet's internal scopeID helper.
func poolSpecID(spec fleet.CycleSpec) string {
	if len(spec.Scope) == 0 {
		return ""
	}
	return spec.Scope[0]
}

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

func TestDispatchPoolIteration_BackfillsReplacementWhileSiblingStillRunning(t *testing.T) {
	fc := policy.FleetConfig{Count: 2, Concurrency: 2, PlanSource: "triage", Scheduling: "pool"}
	backlog := []fleet.Todo{
		{ID: "A", Files: []string{"a.go"}},
		{ID: "B", Files: []string{"b.go"}},
		{ID: "C", Files: []string{"c.go"}},
	}
	planFn := func(context.Context, int) ([]fleet.Todo, error) { return backlog, nil }

	holdA := make(chan struct{})
	holdB := make(chan struct{})
	releaseA := sync.OnceFunc(func() { close(holdA) })
	releaseB := sync.OnceFunc(func() { close(holdB) })
	t.Cleanup(releaseA)
	t.Cleanup(releaseB)
	dispatched := make(chan string, len(backlog))
	launch := func(_ context.Context, spec fleet.CycleSpec) (int, error) {
		id := poolSpecID(spec)
		dispatched <- id
		switch id {
		case "A":
			<-holdA
		case "B":
			<-holdB
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

	// Callback entry order is scheduler-dependent even though RunPool selects A
	// before B, so both are awaited before releasing B.
	releaseB()
	select {
	case id := <-dispatched:
		if id != "C" {
			t.Fatalf("backfill dispatched %q, want C (the only remaining disjoint pending todo)", id)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no replacement lane dispatched for B's exit while sibling A still ran — dispatchPoolIteration did not wire fleet.RunPool (wave barrier still in place)")
	}

	releaseA()
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
