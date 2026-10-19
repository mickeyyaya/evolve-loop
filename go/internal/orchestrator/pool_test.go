package orchestrator

import (
	"context"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/admission"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func cellGraph(t *testing.T, pkgs ...string) *admission.ModuleGraph {
	t.Helper()
	var lines []string
	for _, p := range pkgs {
		lines = append(lines, admission.ModulePath+"/internal/"+p+"|")
	}
	g, err := admission.NewModuleGraph(strings.Join(lines, "\n"), 1000)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func todoIn(id, pkg string) fleet.Todo {
	return fleet.Todo{ID: id, Files: []string{"go/internal/" + pkg + "/f.go"}}
}

func runPool(p Pool, backlog []fleet.Todo) chan []fleet.Result {
	done := make(chan []fleet.Result, 1)
	go func() { done <- p.Run(context.Background(), backlog) }()
	return done
}

func TestOrchestrator_AFreeSlotTakesTheNextAdmissibleTodoAtOnce(t *testing.T) {
	g, events := newGatedLaunch(), &eventLog{}
	pool := Pool{
		Target: 2,
		Admit:  AdmitByCell(admission.Admitter{Graph: cellGraph(t, "inboxrank", "quotastate", "fleet")}),
		Launch: g.launch, Publish: events.publish,
		Leases: NewLeases(leaseConfig(StageEnforce), newFakeClock(), noHeartbeat, events.publish),
	}
	backlog := []fleet.Todo{todoIn("a", "inboxrank"), todoIn("b", "quotastate"), todoIn("d", "inboxrank"), todoIn("c", "fleet")}
	done := runPool(pool, backlog)

	first := []string{waitStart(t, g), waitStart(t, g)}
	if !sameSet(first, []string{"a", "b"}) {
		t.Fatalf("first fill = %v, want a and b (d shares the inboxrank cell with a)", first)
	}
	g.finish("b", 0)
	if next := waitStart(t, g); next != "c" {
		t.Fatalf("after b exits the slot took %q, want c at once (d still waits for a's cell)", next)
	}
	g.finish("a", 0)
	if next := waitStart(t, g); next != "d" {
		t.Fatalf("after a exits the slot took %q, want d: a lane exit frees its cells", next)
	}
	g.finish("c", 0)
	g.finish("d", 0)
	for i, r := range <-done {
		if r.Status() != fleet.LaneOK {
			t.Errorf("result[%d] = %+v, want ok", i, r)
		}
	}
	if got := len(events.withCode(CodeLaneBackfilled)); got != 2 {
		t.Errorf("backfilled events = %d, want 2 (c and d)", got)
	}
	if got := len(events.withCode(CodeLaneLaunched)); got != 2 {
		t.Errorf("launched events = %d, want 2 (a and b)", got)
	}
	if got := len(events.withCode(CodeLaneExited)); got != 4 {
		t.Errorf("exited events = %d, want 4", got)
	}
	if got := len(events.withCode(CodePoolIdle)); got == 0 {
		t.Errorf("idle events = 0, want at least one once the supply ran out")
	}
}

func sameSet(a, b []string) bool {
	m := map[string]int{}
	for _, x := range a {
		m[x]++
	}
	for _, x := range b {
		m[x]--
	}
	for _, v := range m {
		if v != 0 {
			return false
		}
	}
	return len(a) == len(b)
}

func TestLease_ALanePastItsDeadlineIsReleasedAndNotRelaunchedInTheSameRun(t *testing.T) {
	g, events, claims, clock := newGatedLaunch(), &eventLog{}, &claimLog{}, newFakeClock()
	pool := Pool{Target: 1, Launch: g.launch, Publish: events.publish, Claims: claims,
		Resupply: func([]fleet.Todo) []fleet.Todo { return []fleet.Todo{{ID: "A"}} },
		Leases:   NewLeases(leaseConfig(StageEnforce), clock, noHeartbeat, events.publish)}
	done := runPool(pool, []fleet.Todo{{ID: "A"}, {ID: "B"}})
	waitStart(t, g)

	clock.Advance(61 * time.Minute)
	if next := waitStart(t, g); next != "B" {
		t.Fatalf("launch after the stop = %q, want B: the stopped A waits for the next run", next)
	}
	g.finish("B", 0)
	res := <-done
	released, parked, reasons := claims.snapshot()
	if !reflect.DeepEqual(released, []string{"A"}) || len(parked) != 0 {
		t.Errorf("released = %v parked = %v, want A released once and nothing parked", released, parked)
	}
	if len(reasons) != 1 || !strings.Contains(reasons[0], "deadline") {
		t.Errorf("release reasons = %v, want one that names the deadline", reasons)
	}
	stops := events.withCode(CodeLaneDeadline)
	if len(stops) != 1 || stops[0].Fields["action"] != "stopped" {
		t.Errorf("deadline events = %+v, want one stop", stops)
	}
	if len(res) != 2 || res[0].ExitCode != 143 || res[1].Status() != fleet.LaneOK {
		t.Errorf("results = %+v, want the stopped A and the ok B, and no second A", res)
	}
}

func TestLease_ATodoStoppedInTwoRunsIsParkedForTheConsole(t *testing.T) {
	g, events, claims, clock := newGatedLaunch(), &eventLog{}, &claimLog{}, newFakeClock()
	pool := Pool{Target: 1, Launch: g.launch, Publish: events.publish, Claims: claims,
		Leases: NewLeases(leaseConfig(StageEnforce), clock, noHeartbeat, events.publish)}
	for run := 1; run <= 2; run++ {
		done := runPool(pool, []fleet.Todo{{ID: "A"}})
		if id := waitStart(t, g); id != "A" {
			t.Fatalf("run %d launch = %q, want A", run, id)
		}
		clock.Advance(61 * time.Minute)
		<-done
	}
	released, parked, reasons := claims.snapshot()
	if !reflect.DeepEqual(released, []string{"A", "A"}) || !reflect.DeepEqual(parked, []string{"A"}) {
		t.Errorf("released = %v parked = %v, want A released twice and parked once", released, parked)
	}
	if reasons[len(reasons)-1] != ParkReason {
		t.Errorf("park reason = %q, want %q", reasons[len(reasons)-1], ParkReason)
	}
}

func TestPool_ResupplyFeedsTheFreedSlotFromThePort(t *testing.T) {
	var launched []string
	events := &eventLog{}
	pool := Pool{Target: 1, Publish: events.publish,
		Resupply: func([]fleet.Todo) []fleet.Todo { return []fleet.Todo{{ID: "fresh"}} },
		Launch: func(_ context.Context, s fleet.CycleSpec) (int, error) {
			launched = append(launched, s.Scope[0])
			return 0, nil
		}}
	res := pool.Run(context.Background(), []fleet.Todo{{ID: "first"}})
	if !reflect.DeepEqual(launched, []string{"first", "fresh"}) || len(res) != 2 {
		t.Errorf("launched = %v results = %d, want first then the resupplied fresh, with two results", launched, len(res))
	}
	got := events.withCode(CodePoolResupplied)
	if len(got) != 1 || got[0].Fields["todo"] != "fresh" || got[0].Severity != signalcenter.SeverityInfo {
		t.Errorf("resupplied events = %+v, want one INFO event for fresh", got)
	}
}

func TestPool_ShadowLeasesNeverStopOrRelease(t *testing.T) {
	g, events, claims, clock := newGatedLaunch(), &eventLog{}, &claimLog{}, newFakeClock()
	pool := Pool{Target: 1, Launch: g.launch, Publish: events.publish, Claims: claims,
		Leases: NewLeases(leaseConfig(StageShadow), clock, noHeartbeat, events.publish)}
	done := runPool(pool, []fleet.Todo{{ID: "A"}})
	waitStart(t, g)
	clock.Advance(3 * time.Hour)
	g.finish("A", 0)
	<-done
	if released, parked, _ := claims.snapshot(); len(released)+len(parked) != 0 {
		t.Errorf("released = %v parked = %v, want nothing in shadow", released, parked)
	}
}

type failingClaims struct{}

func (failingClaims) Release(string, string) error { return context.DeadlineExceeded }
func (failingClaims) Park(string, string) error    { return context.Canceled }

func TestPool_ARecoveryErrorIsPublishedLoudly(t *testing.T) {
	g, events, clock := newGatedLaunch(), &eventLog{}, newFakeClock()
	pool := Pool{Target: 1, Launch: g.launch, Publish: events.publish, Claims: failingClaims{},
		Leases: NewLeases(leaseConfig(StageEnforce), clock, noHeartbeat, events.publish)}
	for range 2 {
		done := runPool(pool, []fleet.Todo{{ID: "A"}})
		waitStart(t, g)
		clock.Advance(61 * time.Minute)
		<-done
	}
	got := events.withCode(CodeLaneRecoveryFailed)
	if len(got) != 3 {
		t.Fatalf("recovery-failed events = %+v, want 3 (two releases and one park)", got)
	}
	if got[2].Fields["step"] != "park" || got[0].Fields["step"] != "release" {
		t.Errorf("recovery-failed steps = %q, %q, want release then park", got[0].Fields["step"], got[2].Fields["step"])
	}
}

func TestPool_FreshenDropsAStaleTodoAndRewritesTheSpec(t *testing.T) {
	var specs []fleet.CycleSpec
	pool := Pool{Target: 1,
		Freshen: func(s fleet.CycleSpec) (fleet.CycleSpec, bool) {
			if s.Scope[0] == "stale" {
				return fleet.CycleSpec{}, false
			}
			s.GoalHash = "fresh"
			return s, true
		},
		Launch: func(_ context.Context, s fleet.CycleSpec) (int, error) {
			specs = append(specs, s)
			return 0, nil
		},
	}
	res := pool.Run(context.Background(), []fleet.Todo{{ID: "stale"}, {ID: "B"}})
	if len(specs) != 1 || specs[0].Scope[0] != "B" || specs[0].GoalHash != "fresh" {
		t.Errorf("launched = %+v, want only B with the freshened spec", specs)
	}
	if res[0].Status() != fleet.LaneFailed {
		t.Errorf("stale result = %+v, want failed (never launched)", res[0])
	}
}

func TestPool_TheRunEndsAtRunMaxLanesOrOnTheOperatorStop(t *testing.T) {
	count := 0
	launch := func(context.Context, fleet.CycleSpec) (int, error) { count++; return 0, nil }
	backlog := []fleet.Todo{{ID: "A"}, {ID: "B"}, {ID: "C"}}
	Pool{Target: 1, MaxLanes: 2, Launch: launch}.Run(context.Background(), backlog)
	if count != 2 {
		t.Errorf("launches with run_max_lanes 2 = %d, want 2", count)
	}
	count = 0
	Pool{Target: 1, Launch: launch, Halted: func() bool { return count > 0 }}.Run(context.Background(), backlog)
	if count != 1 {
		t.Errorf("launches after the operator stop = %d, want 1 (the lane in flight only)", count)
	}
}

func TestPool_TransitionEventsAreInfoOnTheLoopModule(t *testing.T) {
	events := &eventLog{}
	Pool{Target: 1, Publish: events.publish, Launch: func(context.Context, fleet.CycleSpec) (int, error) { return 0, nil }}.
		Run(context.Background(), []fleet.Todo{{ID: "A"}})
	got := events.withCode(CodeLaneLaunched)
	if len(got) != 1 {
		t.Fatalf("launched events = %+v, want 1", got)
	}
	e := got[0]
	if e.Module != signalcenter.ModuleLoop || e.Kind != signalcenter.KindLoopWave || e.Severity != signalcenter.SeverityInfo ||
		e.Fields["todo"] != "A" || e.Fields["live"] != "1" || e.Fields["target"] != "1" {
		t.Errorf("launched event = %+v, want an INFO loop.wave event with todo, live and target", e)
	}
}

func TestAdmitByCell_TakesTheFirstAdmissibleTodoInRankOrder(t *testing.T) {
	admit := AdmitByCell(admission.Admitter{Graph: cellGraph(t, "x", "y")})
	inFlight := []fleet.Todo{todoIn("held", "x")}
	pending := []fleet.Todo{todoIn("p1", "x"), todoIn("p2", "y"), todoIn("p3", "y")}
	if got := admit(inFlight, pending); got != 1 {
		t.Errorf("admit = %d, want 1 (p1 shares x with the lane in flight)", got)
	}
	if got := admit(inFlight, pending[:1]); got != -1 {
		t.Errorf("admit with only a blocked todo = %d, want -1", got)
	}
}

type reapLog struct {
	mu    sync.Mutex
	lanes []Lane
}

func (r *reapLog) reap(lane Lane) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lanes = append(r.lanes, lane)
}

func (r *reapLog) keys() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for _, l := range r.lanes {
		out = append(out, l.Key)
	}
	return out
}

func TestPool_AStoppedLaneIsReapedOnceAndAnUnstoppedLaneNever(t *testing.T) {
	g, events, claims, clock, reaped := newGatedLaunch(), &eventLog{}, &claimLog{}, newFakeClock(), &reapLog{}
	pool := Pool{Target: 1, Launch: g.launch, Publish: events.publish, Claims: claims, Reap: reaped.reap,
		Leases: NewLeases(leaseConfig(StageEnforce), clock, noHeartbeat, events.publish)}
	done := runPool(pool, []fleet.Todo{{ID: "slow"}, {ID: "quick"}})
	waitStart(t, g)
	clock.Advance(61 * time.Minute)
	if next := waitStart(t, g); next != "quick" {
		t.Fatalf("launch after the stop = %q, want quick", next)
	}
	if got := reaped.keys(); !reflect.DeepEqual(got, []string{"slow"}) {
		t.Fatalf("reaped = %v after the stop, want exactly one reap of slow", got)
	}
	g.finish("quick", 0)
	<-done
	if got := reaped.keys(); !reflect.DeepEqual(got, []string{"slow"}) {
		t.Errorf("reaped = %v, want no reap for the lane that exited by itself", got)
	}
}

func TestPool_ShadowLeasesNeverReap(t *testing.T) {
	g, clock, reaped := newGatedLaunch(), newFakeClock(), &reapLog{}
	pool := Pool{Target: 1, Launch: g.launch, Claims: &claimLog{}, Reap: reaped.reap,
		Leases: NewLeases(leaseConfig(StageShadow), clock, noHeartbeat, nil)}
	done := runPool(pool, []fleet.Todo{{ID: "A"}})
	waitStart(t, g)
	clock.Advance(3 * time.Hour)
	g.finish("A", 0)
	<-done
	if got := reaped.keys(); len(got) != 0 {
		t.Errorf("reaped = %v, want none: a shadow lease stops nothing", got)
	}
}

func TestPool_WithNoLaneInFlightTheTopTodoIsAdmitted(t *testing.T) {
	var launched []string
	pool := Pool{Target: 2,
		Admit: func(_, _ []fleet.Todo) int { return -1 },
		Launch: func(_ context.Context, s fleet.CycleSpec) (int, error) {
			launched = append(launched, s.Scope[0])
			return 0, nil
		}}
	res := pool.Run(context.Background(), []fleet.Todo{{ID: "top"}, {ID: "next"}})
	if !reflect.DeepEqual(launched, []string{"top", "next"}) {
		t.Errorf("launched = %v, want top then next: nothing conflicts with an empty set", launched)
	}
	for i, r := range res {
		if r.Status() != fleet.LaneOK {
			t.Errorf("result[%d] = %+v, want ok", i, r)
		}
	}
}

func TestPool_AStoppedLaneWithNoClaimsIsReapedAndNotTriedAgain(t *testing.T) {
	g, clock, reaped := newGatedLaunch(), newFakeClock(), &reapLog{}
	pool := Pool{Target: 1, Launch: g.launch, Reap: reaped.reap,
		Leases: NewLeases(leaseConfig(StageEnforce), clock, noHeartbeat, nil)}
	done := runPool(pool, []fleet.Todo{{ID: "A"}})
	waitStart(t, g)
	clock.Advance(61 * time.Minute)
	res := <-done
	if got := reaped.keys(); !reflect.DeepEqual(got, []string{"A"}) {
		t.Errorf("reaped = %v, want one reap of A", got)
	}
	if res[0].ExitCode != 143 {
		t.Errorf("result = %+v, want the stopped lane's own exit with no second try", res[0])
	}
}
