package fleet

import (
	"context"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

type heldPool struct {
	hold     *FloorHold
	launched chan string
	done     chan []Result
	cancel   context.CancelFunc
	halted   atomic.Bool
	mu       sync.Mutex
	trans    []PoolTransition
}

func startHeldPool(t *testing.T, cfg PoolConfig, backlog []Todo, launch LaunchFn) *heldPool {
	t.Helper()
	hp := &heldPool{hold: NewFloorHold(), launched: make(chan string, 16), done: make(chan []Result, 1)}
	hp.hold.Set(true)
	ctx, cancel := context.WithCancel(context.Background())
	hp.cancel = cancel
	t.Cleanup(cancel)
	cfg.Hold = hp.hold
	cfg.Halted = hp.halted.Load
	if launch == nil {
		launch = func(_ context.Context, spec CycleSpec) (int, error) {
			hp.launched <- scopeID(spec)
			return 0, nil
		}
	}
	go func() {
		hp.done <- RunPool(ctx, cfg, backlog, launch, func(tr PoolTransition) {
			hp.mu.Lock()
			hp.trans = append(hp.trans, tr)
			hp.mu.Unlock()
		})
	}()
	return hp
}

func (hp *heldPool) transitions() []PoolTransition {
	hp.mu.Lock()
	defer hp.mu.Unlock()
	return append([]PoolTransition(nil), hp.trans...)
}

func (hp *heldPool) expectStillWaiting(t *testing.T) {
	t.Helper()
	select {
	case res := <-hp.done:
		t.Fatalf("the pool returned %d results while held with no lane", len(res))
	case id := <-hp.launched:
		t.Fatalf("the pool launched %s while held", id)
	case <-time.After(100 * time.Millisecond):
	}
}

func (hp *heldPool) expectReturn(t *testing.T) []Result {
	t.Helper()
	select {
	case res := <-hp.done:
		return res
	case <-time.After(2 * time.Second):
		t.Fatal("the pool did not return")
		return nil
	}
}

func TestPool_HoldWithNoRunningLaneStartsNothingAndDoesNotReturn(t *testing.T) {
	hp := startHeldPool(t, PoolConfig{Target: 2}, []Todo{{ID: "A", Files: []string{"a.go"}}}, nil)
	hp.expectStillWaiting(t)
	hp.hold.Set(false)
	select {
	case id := <-hp.launched:
		if id != "A" {
			t.Fatalf("launched %s, want A", id)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("a clear did not wake the pool")
	}
	if res := hp.expectReturn(t); len(res) != 1 || res[0].Err != nil {
		t.Fatalf("results = %+v", res)
	}
}

func TestPool_ACancelWhileHeldWithNoLaneReturns(t *testing.T) {
	hp := startHeldPool(t, PoolConfig{Target: 2}, []Todo{{ID: "A"}}, nil)
	hp.expectStillWaiting(t)
	hp.cancel()
	res := hp.expectReturn(t)
	if len(res) != 1 || res[0].Err != ErrNeverLaunched {
		t.Fatalf("results = %+v, want the todo never launched", res)
	}
}

func TestPool_HaltedWhileHeldWithNoLaneReturnsAfterTheNextWake(t *testing.T) {
	hp := startHeldPool(t, PoolConfig{Target: 2}, []Todo{{ID: "A"}}, nil)
	hp.expectStillWaiting(t)
	hp.halted.Store(true)
	hp.hold.Poke()
	res := hp.expectReturn(t)
	if len(res) != 1 || res[0].Err != ErrNeverLaunched {
		t.Fatalf("results = %+v", res)
	}
}

func TestPool_ACappedPoolWhileHeldWithNoLaneReturns(t *testing.T) {
	backlog := []Todo{{ID: "A", Files: []string{"a.go"}}, {ID: "B", Files: []string{"b.go"}}}
	hold := NewFloorHold()
	launch := func(_ context.Context, spec CycleSpec) (int, error) {
		hold.Set(true)
		return 0, nil
	}
	done := make(chan []Result, 1)
	go func() {
		done <- RunPool(context.Background(), PoolConfig{Target: 1, MaxLaunches: 1, Hold: hold}, backlog, launch, nil)
	}()
	select {
	case res := <-done:
		if res[0].Err != nil || res[1].Err != ErrNeverLaunched {
			t.Fatalf("results = %+v", res)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("a pool at its launch cap waited on the hold")
	}
}

func cpuTime(t *testing.T) time.Duration {
	t.Helper()
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		t.Fatal(err)
	}
	return time.Duration(ru.Utime.Nano() + ru.Stime.Nano())
}

func TestPool_ACancelWithTwoRunningLanesReturnsAfterBothCompleteWithoutSpinning(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{}, 2)
	launch := func(_ context.Context, _ CycleSpec) (int, error) {
		started <- struct{}{}
		<-release
		return 0, nil
	}
	hold := NewFloorHold()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan []Result, 1)
	go func() {
		done <- RunPool(ctx, PoolConfig{Target: 2, Hold: hold}, []Todo{{ID: "A", Files: []string{"a.go"}}, {ID: "B", Files: []string{"b.go"}}}, launch, nil)
	}()
	<-started
	<-started
	cancel()
	before := cpuTime(t)
	select {
	case <-done:
		t.Fatal("the pool returned with two lanes still running")
	case <-time.After(300 * time.Millisecond):
	}
	if spent := cpuTime(t) - before; spent > 150*time.Millisecond {
		t.Fatalf("a cancel with running lanes burned %v of CPU in 300ms: the done case spins", spent)
	}
	close(release)
	select {
	case res := <-done:
		if len(res) != 2 || res[0].Err != nil || res[1].Err != nil {
			t.Fatalf("results = %+v", res)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the pool did not return after both lanes completed")
	}
}

func TestPool_HoldLetsRunningLanesFinishAndStartsNoOtherLane(t *testing.T) {
	release := make(chan struct{})
	started := make(chan string, 4)
	hold := NewFloorHold()
	launch := func(_ context.Context, spec CycleSpec) (int, error) {
		started <- scopeID(spec)
		<-release
		return 0, nil
	}
	backlog := []Todo{{ID: "A", Files: []string{"a.go"}}, {ID: "B", Files: []string{"b.go"}}, {ID: "C", Files: []string{"c.go"}}}
	done := make(chan []Result, 1)
	go func() { done <- RunPool(context.Background(), PoolConfig{Target: 2, Hold: hold}, backlog, launch, nil) }()
	<-started
	<-started
	hold.Set(true)
	close(release)
	select {
	case <-started:
		t.Fatal("a third lane started under the hold")
	case <-time.After(150 * time.Millisecond):
	}
	hold.Set(false)
	select {
	case id := <-started:
		if id != "C" {
			t.Fatalf("started %s, want C", id)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the clear did not start the held todo")
	}
	<-done
}

func TestPool_TheHoldTransitionsOnceAtTheStartAndOnceAtTheClear(t *testing.T) {
	hp := startHeldPool(t, PoolConfig{Target: 1}, []Todo{{ID: "A"}}, nil)
	hp.expectStillWaiting(t)
	hp.hold.Poke()
	hp.hold.Poke()
	hp.hold.Set(false)
	<-hp.launched
	hp.expectReturn(t)
	var held []bool
	for _, tr := range hp.transitions() {
		if tr.Kind == PoolFloorHold {
			held = append(held, tr.Held)
		}
	}
	if len(held) != 2 || !held[0] || held[1] {
		t.Fatalf("floor hold transitions = %v, want [true false]", held)
	}
}

func TestPool_AHoldedSlotReportsIdleWithTheHoldFlag(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{}, 1)
	hold := NewFloorHold()
	var mu sync.Mutex
	var idle []PoolTransition
	launch := func(_ context.Context, _ CycleSpec) (int, error) {
		started <- struct{}{}
		<-release
		return 0, nil
	}
	idled := make(chan struct{}, 4)
	cfg := PoolConfig{Target: 1, Hold: hold, Resupply: func([]Todo) []Todo { return nil }}
	done := make(chan []Result, 1)
	go func() {
		done <- RunPool(context.Background(), cfg, []Todo{{ID: "A"}}, launch, func(tr PoolTransition) {
			mu.Lock()
			defer mu.Unlock()
			if tr.Kind == PoolIdle {
				idle = append(idle, tr)
				idled <- struct{}{}
			}
		})
	}()
	<-started
	hold.Set(true)
	close(release)
	<-idled
	hold.Set(false)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("pool did not return")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(idle) == 0 || !idle[0].Held {
		t.Fatalf("idle transitions = %+v, want the first to carry the hold", idle)
	}
}

func TestPool_AClosedWakeChannelIsIgnoredAndNeverSpinsTheFill(t *testing.T) {
	wake := make(chan struct{})
	close(wake)
	hold := &closedWake{wake: wake}
	cfg := PoolConfig{Target: 1, Hold: hold}
	release := make(chan struct{})
	started := make(chan struct{})
	launch := func(_ context.Context, _ CycleSpec) (int, error) { close(started); <-release; return 0, nil }
	done := make(chan []Result, 1)
	go func() { done <- RunPool(context.Background(), cfg, []Todo{{ID: "A"}}, launch, nil) }()
	<-started
	<-time.After(100 * time.Millisecond)
	if n := hold.reads.Load(); n > 5 {
		t.Fatalf("the pool read the hold %d times behind a closed wake channel, want no spin: a closed channel is always ready", n)
	}
	close(release)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("a closed wake channel stopped the pool from returning")
	}
}

type closedWake struct {
	wake  chan struct{}
	reads atomic.Int64
}

func (c *closedWake) Held() bool {
	c.reads.Add(1)
	return false
}

func (c *closedWake) Wake() <-chan struct{} { return c.wake }

func TestFloorHold_SetStoresTheStateAndWakesOnce(t *testing.T) {
	t.Parallel()
	h := NewFloorHold()
	if h.Held() {
		t.Fatal("a new hold is held")
	}
	h.Set(true)
	h.Set(true)
	if !h.Held() {
		t.Fatal("Set(true) did not hold")
	}
	n := 0
	for {
		select {
		case <-h.Wake():
			n++
			continue
		default:
		}
		break
	}
	if n != 1 {
		t.Fatalf("woke %d times, want one coalesced wake", n)
	}
	h.Set(false)
	if h.Held() {
		t.Fatal("Set(false) did not clear")
	}
}

func TestPool_AClearWithAnEmptyListResuppliesAndStartsTheNewTodo(t *testing.T) {
	var supplied atomic.Bool
	cfg := PoolConfig{Target: 1, Resupply: func([]Todo) []Todo {
		supplied.Store(true)
		return []Todo{{ID: "N"}}
	}}
	hp := startHeldPool(t, cfg, nil, nil)
	hp.expectStillWaiting(t)
	hp.hold.Set(false)
	select {
	case id := <-hp.launched:
		if id != "N" || !supplied.Load() {
			t.Fatalf("launched %q supplied %v", id, supplied.Load())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the clear did not resupply and launch")
	}
	hp.expectReturn(t)
}

type clearsOnWake struct {
	cleared atomic.Bool
	wake    chan struct{}
}

func (c *clearsOnWake) Held() bool { return !c.cleared.Load() }

func (c *clearsOnWake) Wake() <-chan struct{} {
	c.cleared.Store(true)
	return c.wake
}

func TestPool_AClearBetweenTheFirstFillAndTheLoopStillStartsTheTodo(t *testing.T) {
	t.Parallel()
	hold := &clearsOnWake{wake: make(chan struct{}, 1)}
	hold.wake <- struct{}{}
	launched := make(chan string, 1)
	launch := func(_ context.Context, spec CycleSpec) (int, error) {
		launched <- scopeID(spec)
		return 0, nil
	}
	done := make(chan []Result, 1)
	go func() {
		done <- RunPool(context.Background(), PoolConfig{Target: 1, Hold: hold}, []Todo{{ID: "A"}}, launch, nil)
	}()
	select {
	case res := <-done:
		if res[0].Err != nil {
			t.Fatalf("the todo was never launched: a clear that arrived before the loop ended the pool (%+v)", res)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the pool did not return")
	}
}
