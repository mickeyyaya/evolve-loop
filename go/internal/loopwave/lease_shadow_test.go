package loopwave

import (
	"context"
	"reflect"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/orchestrator"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type specLog struct {
	mu    sync.Mutex
	specs []fleet.CycleSpec
}

func (l *specLog) launch(wait func(key string)) fleet.LaunchFn {
	return func(_ context.Context, spec fleet.CycleSpec) (int, error) {
		wait(spec.Scope[0])
		l.mu.Lock()
		l.specs = append(l.specs, spec)
		l.mu.Unlock()
		return 0, nil
	}
}

func (l *specLog) sorted() []fleet.CycleSpec {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := append([]fleet.CycleSpec(nil), l.specs...)
	sort.Slice(out, func(i, j int) bool { return out[i].Scope[0] < out[j].Scope[0] })
	return out
}

func TestWave_AShadowLeaseKeepsTheBarrierLaunchesExactlyAsToday(t *testing.T) {
	plan := []fleet.CycleSpec{{Scope: []string{"a"}}, {Scope: []string{"b"}}, {Scope: []string{"c"}}}

	today := newHarness(t)
	for _, id := range []string{"a", "b", "c"} {
		lifecycleItem(t, today.evolveDir, inboxmover.StatePending, id)
	}
	var plain specLog
	plainResults := today.e.Launcher(1, 3, plain.launch(func(string) {})).Run(context.Background(), plan)

	var mu sync.Mutex
	seen := map[string]chan struct{}{"a": make(chan struct{}), "b": make(chan struct{}), "c": make(chan struct{})}
	publish := func(e signalcenter.Event) {
		if e.Code != orchestrator.CodeLaneDeadline || e.Fields["stage"] != orchestrator.StageShadow {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if ch, ok := seen[e.Fields["todo"]]; ok {
			close(ch)
			delete(seen, e.Fields["todo"])
		}
	}
	waitDeadline := func(key string) {
		mu.Lock()
		ch, ok := seen[key]
		mu.Unlock()
		if !ok {
			return
		}
		select {
		case <-ch:
		case <-time.After(5 * time.Second):
			t.Errorf("lane %s never saw its shadow deadline", key)
		}
	}
	leases := orchestrator.NewLeases(orchestrator.LeaseConfig{Stage: orchestrator.StageShadow, Deadline: time.Nanosecond, Stall: time.Hour},
		orchestrator.SystemClock{}, func(orchestrator.Lane) (time.Time, bool) { return time.Time{}, false }, publish)
	shadow := newHarness(t)
	for _, id := range []string{"a", "b", "c"} {
		lifecycleItem(t, shadow.evolveDir, inboxmover.StatePending, id)
	}
	var leased specLog
	leasedResults := shadow.e.Launcher(1, 3, leases.Watch(leased.launch(waitDeadline), nil)).Run(context.Background(), plan)

	if !reflect.DeepEqual(leased.sorted(), plain.sorted()) {
		t.Errorf("launches with a shadow lease =\n%+v\nwant today's\n%+v", leased.sorted(), plain.sorted())
	}
	if !reflect.DeepEqual(leasedResults, plainResults) {
		t.Errorf("results with a shadow lease = %+v, want today's %+v: a passed shadow deadline stops nothing", leasedResults, plainResults)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(seen) != 0 {
		t.Errorf("lanes with no shadow deadline event: %v, want every lane leased", seen)
	}
}

func TestShouldRunWave_TheEnforceStageHandsTheFleetToThePool(t *testing.T) {
	fc := policy.FleetConfig{Count: 4, PlanSource: "triage", Scheduling: "wave", Orchestrator: policy.OrchestratorConfig{Stage: "shadow"}}
	if !ShouldRunWave(fc) {
		t.Fatalf("ShouldRunWave(shadow) = false, want the barrier wave as today")
	}
	fc.Orchestrator.Stage = "enforce"
	if ShouldRunWave(fc) {
		t.Errorf("ShouldRunWave(enforce) = true, want false: the orchestrator pool runs the lanes")
	}
	if !ShouldRunPool(fc) {
		t.Errorf("ShouldRunPool(enforce) = false, want true")
	}
	fc.Orchestrator.Stage, fc.Scheduling = "shadow", "pool"
	if ShouldRunWave(fc) || !ShouldRunPool(fc) {
		t.Errorf("scheduling pool at shadow: wave=%v pool=%v, want the pool as today", ShouldRunWave(fc), ShouldRunPool(fc))
	}
	fc.Count = 1
	if ShouldRunPool(fc) {
		t.Errorf("ShouldRunPool(count 1) = true, want false")
	}
}
