package main

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestShouldRunPool_DegenerateAndBoundaryCounts(t *testing.T) {
	cases := []struct {
		name string
		fc   policy.FleetConfig
		want bool
	}{
		{"count zero", policy.FleetConfig{Count: 0, PlanSource: "triage", Scheduling: "pool"}, false},
		{"count negative", policy.FleetConfig{Count: -5, PlanSource: "triage", Scheduling: "pool"}, false},
		{"count minimal valid boundary (2)", policy.FleetConfig{Count: 2, PlanSource: "triage", Scheduling: "pool"}, true},
		{"count large-scale (100000)", policy.FleetConfig{Count: 100000, PlanSource: "triage", Scheduling: "pool"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldRunPool(tc.fc); got != tc.want {
				t.Errorf("shouldRunPool(%+v) = %v, want %v", tc.fc, got, tc.want)
			}
		})
	}
}

func TestShouldRunPool_ExactStringMatchRequired(t *testing.T) {
	mismatches := []policy.FleetConfig{
		{Count: 2, PlanSource: "triage", Scheduling: "Pool"},
		{Count: 2, PlanSource: "triage", Scheduling: "POOL"},
		{Count: 2, PlanSource: "triage", Scheduling: " pool"},
		{Count: 2, PlanSource: "triage", Scheduling: "pool "},
		{Count: 2, PlanSource: "Triage", Scheduling: "pool"},
		{Count: 2, PlanSource: "triage ", Scheduling: "pool"},
	}
	for _, fc := range mismatches {
		if shouldRunPool(fc) {
			t.Errorf("shouldRunPool(%+v) = true, want false — comparisons must be exact-match, not fuzzy/case-insensitive", fc)
		}
		if fc.Scheduling != "pool" && !shouldRunWave(fc) {
			t.Errorf("shouldRunWave(%+v) = false, want true — a non-exact-\"pool\" Scheduling must fall back to the wave path, never dispatch nowhere", fc)
		}
	}
}

func TestDispatchPoolIteration_SingleLaneCountInertNoLaunch(t *testing.T) {
	fc := policy.FleetConfig{Count: 1, Concurrency: 1, PlanSource: "triage", Scheduling: "pool"}
	planCalled, launched := false, 0
	planFn := func(context.Context, int) ([]fleet.Todo, error) {
		planCalled = true
		return []fleet.Todo{{ID: "A", Files: []string{"a.go"}}}, nil
	}
	launch := func(context.Context, fleet.CycleSpec) (int, error) { launched++; return 0, nil }

	ran, _, _, err := dispatchPoolIteration(context.Background(), fc, func() error { return nil }, planFn, launch, 0)
	if err != nil {
		t.Fatalf("dispatchPoolIteration returned error: %v, want nil on the gated-off path", err)
	}
	if ran {
		t.Fatalf("dispatchPoolIteration reported ran=true for a single-lane (Count=1) fleet — the pool gate's Count>1 clause must be enforced internally")
	}
	if planCalled {
		t.Errorf("planFn invoked for a Count=1 fleet — the gate must short-circuit BEFORE planning")
	}
	if launched != 0 {
		t.Errorf("launch invoked %d times for a Count=1 fleet, want 0", launched)
	}
}

func TestDispatchPoolIteration_ManualPlanSourceInertNoLaunch(t *testing.T) {
	fc := policy.FleetConfig{Count: 2, Concurrency: 2, PlanSource: "manual", Scheduling: "pool"}
	planCalled, launched := false, 0
	planFn := func(context.Context, int) ([]fleet.Todo, error) {
		planCalled = true
		return []fleet.Todo{{ID: "A", Files: []string{"a.go"}}}, nil
	}
	launch := func(context.Context, fleet.CycleSpec) (int, error) { launched++; return 0, nil }

	ran, _, _, err := dispatchPoolIteration(context.Background(), fc, func() error { return nil }, planFn, launch, 0)
	if err != nil {
		t.Fatalf("dispatchPoolIteration returned error: %v, want nil on the gated-off path", err)
	}
	if ran {
		t.Fatalf("dispatchPoolIteration reported ran=true for a manual-plan-source fleet — the pool gate's PlanSource==\"triage\" clause must be enforced internally")
	}
	if planCalled || launched != 0 {
		t.Errorf("planCalled=%v launched=%d, want false/0 — a manual-plan-source fleet must never plan or launch through the pool seam", planCalled, launched)
	}
}

func TestDispatchPoolIteration_PlanFnErrorSurfacesNoLaunch(t *testing.T) {
	fc := policy.FleetConfig{Count: 2, Concurrency: 2, PlanSource: "triage", Scheduling: "pool"}
	wantErr := errors.New("triage phase failed")
	launched := 0
	planFn := func(context.Context, int) ([]fleet.Todo, error) { return nil, wantErr }
	launch := func(context.Context, fleet.CycleSpec) (int, error) { launched++; return 0, nil }

	ran, _, _, err := dispatchPoolIteration(context.Background(), fc, func() error { return nil }, planFn, launch, 0)
	if !errors.Is(err, wantErr) {
		t.Fatalf("dispatchPoolIteration error = %v, want it to wrap the planFn error %v", err, wantErr)
	}
	if ran {
		t.Errorf("dispatchPoolIteration reported ran=true despite a planFn error")
	}
	if launched != 0 {
		t.Errorf("launch invoked %d times despite a planFn error, want 0", launched)
	}
}

func TestDispatchPoolIteration_BacklogShorterThanTargetNeverBlocks(t *testing.T) {
	fc := policy.FleetConfig{Count: 5, Concurrency: 5, PlanSource: "triage", Scheduling: "pool"}
	backlog := []fleet.Todo{
		{ID: "A", Files: []string{"a.go"}},
		{ID: "B", Files: []string{"b.go"}},
	}
	planFn := func(context.Context, int) ([]fleet.Todo, error) { return backlog, nil }
	launch := func(context.Context, fleet.CycleSpec) (int, error) { return 0, nil }

	done := make(chan struct {
		ran     bool
		results []fleet.Result
		err     error
	}, 1)
	go func() {
		ran, _, results, err := dispatchPoolIteration(context.Background(), fc, func() error { return nil }, planFn, launch, 0)
		done <- struct {
			ran     bool
			results []fleet.Result
			err     error
		}{ran, results, err}
	}()

	select {
	case o := <-done:
		if o.err != nil {
			t.Fatalf("dispatchPoolIteration returned error: %v, want nil", o.err)
		}
		if !o.ran {
			t.Fatalf("dispatchPoolIteration reported ran=false for a non-empty backlog smaller than Target")
		}
		if len(o.results) != 2 {
			t.Fatalf("len(results) = %d, want 2 (a Target larger than the backlog must not block or fabricate phantom lanes)", len(o.results))
		}
	case <-time.After(3 * time.Second):
		t.Fatal("dispatchPoolIteration blocked waiting for a Target wider than the available disjoint backlog — RunPool must run fewer lanes, never stall")
	}
}

func TestDispatchPoolIteration_LargeScaleBacklogAllDispatchedExactlyOnce(t *testing.T) {
	const n = 30
	fc := policy.FleetConfig{Count: 3, Concurrency: 3, PlanSource: "triage", Scheduling: "pool"}
	backlog := make([]fleet.Todo, n)
	for i := range backlog {
		id := fmt.Sprintf("T%02d", i)
		backlog[i] = fleet.Todo{ID: id, Files: []string{id + ".go"}}
	}
	planFn := func(context.Context, int) ([]fleet.Todo, error) { return backlog, nil }

	seenMu := make(chan map[string]int, 1)
	seen := map[string]int{}
	seenMu <- seen
	launch := func(_ context.Context, spec fleet.CycleSpec) (int, error) {
		m := <-seenMu
		id := ""
		if len(spec.Scope) > 0 {
			id = spec.Scope[0]
		}
		m[id]++
		seenMu <- m
		return 0, nil
	}

	done := make(chan struct {
		ran     bool
		results []fleet.Result
		err     error
	}, 1)
	go func() {
		ran, _, results, err := dispatchPoolIteration(context.Background(), fc, func() error { return nil }, planFn, launch, 0)
		done <- struct {
			ran     bool
			results []fleet.Result
			err     error
		}{ran, results, err}
	}()

	select {
	case o := <-done:
		if o.err != nil {
			t.Fatalf("dispatchPoolIteration returned error: %v, want nil", o.err)
		}
		if !o.ran {
			t.Fatalf("dispatchPoolIteration reported ran=false for a %d-item backlog", n)
		}
		if len(o.results) != n {
			t.Fatalf("len(results) = %d, want %d (one result per backlog item, no drops/dupes under churn)", len(o.results), n)
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("dispatchPoolIteration did not complete a %d-item backlog under a narrow Target=3 pool within 10s", n)
	}
	m := <-seenMu
	if len(m) != n {
		t.Errorf("launch was invoked for %d distinct todo ids, want %d — every backlog item must be dispatched exactly once", len(m), n)
	}
	for id, count := range m {
		if count != 1 {
			t.Errorf("todo id %q was launched %d times, want exactly 1", id, count)
		}
	}
}

func TestDispatchPoolIteration_NonPositiveConcurrencyNeverBlocks(t *testing.T) {
	for _, conc := range []int{0, -1} {
		t.Run(fmt.Sprintf("concurrency=%d", conc), func(t *testing.T) {
			fc := policy.FleetConfig{Count: 3, Concurrency: conc, PlanSource: "triage", Scheduling: "pool"}
			backlog := []fleet.Todo{
				{ID: "A", Files: []string{"a.go"}},
				{ID: "B", Files: []string{"b.go"}},
				{ID: "C", Files: []string{"c.go"}},
			}
			planFn := func(context.Context, int) ([]fleet.Todo, error) { return backlog, nil }
			launch := func(context.Context, fleet.CycleSpec) (int, error) { return 0, nil }

			done := make(chan error, 1)
			var ran bool
			var results []fleet.Result
			go func() {
				var err error
				ran, _, results, err = dispatchPoolIteration(context.Background(), fc, func() error { return nil }, planFn, launch, 0)
				done <- err
			}()
			select {
			case err := <-done:
				if err != nil {
					t.Fatalf("dispatchPoolIteration returned error: %v, want nil", err)
				}
				if !ran {
					t.Fatalf("dispatchPoolIteration reported ran=false for a non-empty backlog with Concurrency=%d", conc)
				}
				if len(results) != 3 {
					t.Fatalf("len(results) = %d, want 3", len(results))
				}
			case <-time.After(3 * time.Second):
				t.Fatalf("dispatchPoolIteration blocked with Concurrency=%d — a non-positive Concurrency must fall back to Target, never stall", conc)
			}
		})
	}
}
