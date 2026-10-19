package fleet

import (
	"context"
	"reflect"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
)

type resupplier struct {
	mu      sync.Mutex
	batches [][]Todo
	calls   int
	held    [][]string
}

func (r *resupplier) next(held []Todo) []Todo {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	r.held = append(r.held, ids(held))
	if len(r.batches) == 0 {
		return nil
	}
	batch := r.batches[0]
	r.batches = r.batches[1:]
	return batch
}

func (r *resupplier) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

type heldLaunch struct {
	hold    chan struct{}
	free    map[string]bool
	live    atomic.Int64
	peak    atomic.Int64
	started chan string
}

func newHeldLaunch(free ...string) *heldLaunch {
	h := &heldLaunch{hold: make(chan struct{}), free: map[string]bool{}, started: make(chan string, 16)}
	for _, id := range free {
		h.free[id] = true
	}
	return h
}

func (h *heldLaunch) launch(_ context.Context, spec CycleSpec) (int, error) {
	id := scopeID(spec)
	live := h.live.Add(1)
	for {
		peak := h.peak.Load()
		if live <= peak || h.peak.CompareAndSwap(peak, live) {
			break
		}
	}
	h.started <- id
	if !h.free[id] {
		<-h.hold
	}
	h.live.Add(-1)
	return 0, nil
}

func (h *heldLaunch) await(t *testing.T, n int) []string {
	t.Helper()
	got := make([]string, 0, n)
	for len(got) < n {
		got = append(got, <-h.started)
	}
	return got
}

func TestRunPool_AResupplyAfterAnExitFillsTheFreeSlotsBackToTheTarget(t *testing.T) {
	supply := &resupplier{batches: [][]Todo{{{ID: "B"}, {ID: "C"}, {ID: "D"}}}}
	h := newHeldLaunch("A")
	done := make(chan []Result, 1)
	go func() {
		done <- RunPool(context.Background(), PoolConfig{Target: 4, Resupply: supply.next}, []Todo{{ID: "A"}, {ID: "X"}}, h.launch, nil)
	}()
	got := h.await(t, 5)
	if want := []string{"A", "X", "B", "C", "D"}; !sameIDs(got, want) {
		t.Fatalf("launches = %v, want %v", got, want)
	}
	if peak := h.peak.Load(); peak != 4 {
		t.Errorf("peak live lanes = %d, want the target 4 after the resupply", peak)
	}
	close(h.hold)
	res := <-done
	if len(res) != 5 {
		t.Fatalf("results = %d, want 5: the results grow with the resupplied backlog", len(res))
	}
	for i, r := range res {
		if r.Index != i || r.Status() != LaneOK {
			t.Errorf("result[%d] = %+v, want an ok result at index %d", i, r, i)
		}
	}
}

func sameIDs(a, b []string) bool {
	a, b = slices.Clone(a), slices.Clone(b)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}

func TestRunPool_AResupplyNeverAddsAnIDTheBacklogAlreadyHolds(t *testing.T) {
	supply := &resupplier{batches: [][]Todo{{{ID: "A"}, {ID: "B"}, {ID: "B"}}}}
	var log launchLog
	res := RunPool(context.Background(), PoolConfig{Target: 2, Resupply: supply.next}, []Todo{{ID: "A"}}, log.launch(0), nil)
	if want := []string{"A", "B"}; !reflect.DeepEqual(log.got(), want) {
		t.Errorf("launches = %v, want %v: A ran once and B is added once", log.got(), want)
	}
	if len(res) != 2 {
		t.Errorf("results = %+v, want one result for A and one for B", res)
	}
	if want := [][]string{{"A"}, {"A", "B"}}; !reflect.DeepEqual(supply.held, want) {
		t.Errorf("held todos given to the port = %v, want %v: the port sees every todo the run holds", supply.held, want)
	}
}

func TestRunPool_AnEmptyResupplyEndsInIdleWithOneCallForEachExit(t *testing.T) {
	supply := &resupplier{}
	var log launchLog
	var kinds []PoolTransitionKind
	var mu sync.Mutex
	RunPool(context.Background(), PoolConfig{Target: 2, Resupply: supply.next}, []Todo{{ID: "A"}, {ID: "B"}}, log.launch(0), func(pt PoolTransition) {
		mu.Lock()
		kinds = append(kinds, pt.Kind)
		mu.Unlock()
	})
	if calls := supply.count(); calls != 2 {
		t.Errorf("resupply calls = %d, want 2: one for each exit and no spin", calls)
	}
	if last := kinds[len(kinds)-1]; last != PoolIdle {
		t.Errorf("last transition = %q, want %q", last, PoolIdle)
	}
}

func TestRunPool_EachResuppliedTodoIsNamedByOneTransition(t *testing.T) {
	supply := &resupplier{batches: [][]Todo{{{ID: "B"}}}}
	var log launchLog
	var got []PoolTransition
	var mu sync.Mutex
	RunPool(context.Background(), PoolConfig{Target: 1, Resupply: supply.next}, []Todo{{ID: "A"}}, log.launch(0), func(pt PoolTransition) {
		mu.Lock()
		got = append(got, pt)
		mu.Unlock()
	})
	want := []PoolTransition{
		{Kind: PoolLaunched, Todo: "A", Live: 1, Target: 1},
		{Kind: PoolExited, Todo: "A", Live: 0, Target: 1},
		{Kind: PoolResupplied, Todo: "B", Live: 0, Target: 1, Pending: 1},
		{Kind: PoolBackfilled, Todo: "B", Live: 1, Target: 1},
		{Kind: PoolExited, Todo: "B", Live: 0, Target: 1},
		{Kind: PoolIdle, Live: 0, Target: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("transitions =\n%+v\nwant\n%+v", got, want)
	}
}

func TestRunPool_AClosedPoolNeverAsksForResupply(t *testing.T) {
	for name, closed := range map[string]func(cfg PoolConfig, log *launchLog) PoolConfig{
		"launch limit": func(cfg PoolConfig, _ *launchLog) PoolConfig { cfg.MaxLaunches = 1; return cfg },
		"halted": func(cfg PoolConfig, log *launchLog) PoolConfig {
			cfg.Halted = func() bool { return len(log.got()) > 0 }
			return cfg
		},
	} {
		t.Run(name, func(t *testing.T) {
			supply := &resupplier{batches: [][]Todo{{{ID: "B"}}}}
			var log launchLog
			cfg := closed(PoolConfig{Target: 2, Resupply: supply.next}, &log)
			RunPool(context.Background(), cfg, []Todo{{ID: "A"}}, log.launch(0), nil)
			if calls := supply.count(); calls != 0 || !reflect.DeepEqual(log.got(), []string{"A"}) {
				t.Errorf("resupply calls = %d launches = %v, want 0 calls and only A when the pool admits nothing new", calls, log.got())
			}
		})
	}
}
