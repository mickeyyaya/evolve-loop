package fleet

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestRunPool_BackfillsReplacementWhileSiblingLaneStillRunning(t *testing.T) {
	backlog := []Todo{
		{ID: "A", Files: []string{"a.go"}},
		{ID: "B", Files: []string{"b.go"}},
		{ID: "C", Files: []string{"c.go"}},
	}
	holdA := make(chan struct{})
	holdB := make(chan struct{})
	releaseA := sync.OnceFunc(func() { close(holdA) })
	releaseB := sync.OnceFunc(func() { close(holdB) })
	t.Cleanup(releaseA)
	t.Cleanup(releaseB)
	dispatched := make(chan string, len(backlog))
	launch := func(_ context.Context, spec CycleSpec) (int, error) {
		id := scopeID(spec)
		dispatched <- id
		switch id {
		case "A":
			<-holdA
		case "B":
			<-holdB
		}
		return 0, nil
	}

	done := make(chan []Result, 1)
	go func() {
		done <- RunPool(context.Background(), PoolConfig{Target: 2}, backlog, launch, nil)
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

	// Let B finish only after observing both initial callbacks: callback entry
	// order is scheduler-dependent even when the pool dispatches A before B.
	releaseB()
	select {
	case id := <-dispatched:
		if id != "C" {
			t.Fatalf("backfill dispatched %q, want C (the only remaining disjoint pending todo)", id)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no replacement lane was dispatched for B's exit while sibling lane A was still running -- the wave barrier was not removed")
	}

	releaseA()
	var results []Result
	select {
	case results = <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RunPool did not return after all 3 backlog lanes finished")
	}
	if len(results) != 3 {
		t.Fatalf("len(results) = %d, want 3 (one per backlog item)", len(results))
	}
	for _, r := range results {
		if r.Err != nil {
			t.Errorf("result[%d] = %+v, want no error", r.Index, r)
		}
	}
}

func TestRunPool_CollidingFilesNeverCoRunButAllEventuallyDispatch(t *testing.T) {
	backlog := []Todo{
		{ID: "A", Files: []string{"shared.go"}},
		{ID: "B", Files: []string{"shared.go"}},
		{ID: "C", Files: []string{"shared.go"}},
	}
	var mu sync.Mutex
	current, maxSeen := 0, 0
	release := make(chan struct{})
	launch := func(_ context.Context, _ CycleSpec) (int, error) {
		mu.Lock()
		current++
		if current > maxSeen {
			maxSeen = current
		}
		mu.Unlock()
		<-release
		mu.Lock()
		current--
		mu.Unlock()
		return 0, nil
	}

	done := make(chan []Result, 1)
	go func() {
		done <- RunPool(context.Background(), PoolConfig{Target: 2}, backlog, launch, nil)
	}()

	for range backlog {
		release <- struct{}{}
	}
	var results []Result
	select {
	case results = <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RunPool hung on an all-colliding-files backlog -- must shrink to 1 lane, never stall")
	}
	if len(results) != 3 {
		t.Fatalf("len(results) = %d, want 3 -- every colliding-file todo must still eventually dispatch (never in-supervisor sequential fallback, per L4; the pool itself must drain the backlog)", len(results))
	}
	mu.Lock()
	defer mu.Unlock()
	if maxSeen > 1 {
		t.Fatalf("max concurrent lanes observed = %d, want 1 -- todos sharing a file must never co-run (cross-lane collision on the shared tree)", maxSeen)
	}
}

func TestRunPool_EmptyBacklogIdlesCleanlyNoLaunchCalls(t *testing.T) {
	calls := 0
	launch := func(_ context.Context, _ CycleSpec) (int, error) {
		calls++
		return 0, nil
	}
	done := make(chan []Result, 1)
	go func() {
		done <- RunPool(context.Background(), PoolConfig{Target: 2}, nil, launch, nil)
	}()
	select {
	case results := <-done:
		if len(results) != 0 {
			t.Errorf("len(results) = %d, want 0 for an empty backlog", len(results))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("RunPool hung on an empty backlog")
	}
	if calls != 0 {
		t.Errorf("launch invoked %d times for an empty backlog, want 0", calls)
	}
}

func TestRunPool_EmitsShrinkAndRecoveryTransitions(t *testing.T) {
	backlog := []Todo{
		{ID: "A", Files: []string{"a.go"}},
		{ID: "B", Files: []string{"b.go"}},
		{ID: "C", Files: []string{"c.go"}},
	}
	holdA := make(chan struct{})
	dispatched := make(chan string, len(backlog))
	launch := func(_ context.Context, spec CycleSpec) (int, error) {
		id := scopeID(spec)
		dispatched <- id
		if id == "A" {
			<-holdA
		}
		return 0, nil
	}

	var mu sync.Mutex
	var transitions []PoolTransition
	onTransition := func(pt PoolTransition) {
		mu.Lock()
		transitions = append(transitions, pt)
		mu.Unlock()
	}

	done := make(chan []Result, 1)
	go func() {
		done <- RunPool(context.Background(), PoolConfig{Target: 2}, backlog, launch, onTransition)
	}()

	seen := map[string]bool{}
	for len(seen) < 2 {
		select {
		case id := <-dispatched:
			seen[id] = true
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for initial fill; got %v", seen)
		}
	}
	select {
	case <-dispatched:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for C's backfill dispatch")
	}
	close(holdA)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RunPool did not return")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(transitions) == 0 {
		t.Fatal("onTransition was never called -- want at least one live-count transition")
	}
	sawFullFill := false
	for _, pt := range transitions {
		if pt.Target != 2 {
			t.Errorf("transition %+v has Target=%d, want 2 (cfg.Target)", pt, pt.Target)
		}
		if pt.Live == 2 {
			sawFullFill = true
		}
	}
	if !sawFullFill {
		t.Errorf("transitions = %+v, want at least one Live=2 transition (the pool reaching full target width, both on initial fill and again after backfilling C)", transitions)
	}
}

func TestRunPool_BackfillPrefersHighestPriorityDisjointCandidate(t *testing.T) {
	backlog := []Todo{
		{ID: "A", Files: []string{"a.go"}},
		{ID: "low", Files: []string{"low.go"}, Priority: 1},
		{ID: "high", Files: []string{"high.go"}, Priority: 9},
	}
	holdA := make(chan struct{})
	dispatched := make(chan string, len(backlog))
	launch := func(_ context.Context, spec CycleSpec) (int, error) {
		id := scopeID(spec)
		dispatched <- id
		if id == "A" {
			<-holdA
			return 0, nil
		}
		return 0, nil
	}

	done := make(chan []Result, 1)
	go func() {
		done <- RunPool(context.Background(), PoolConfig{Target: 1}, backlog, launch, nil)
	}()

	select {
	case id := <-dispatched:
		if id != "A" {
			t.Fatalf("initial dispatch = %q, want A (Target=1, backlog order)", id)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for A's initial dispatch")
	}

	// Target=1 frees one slot at a time, so low and high compete for it and the order is deterministic.
	close(holdA)

	firstOfPair := ""
	for i := 0; i < 2; i++ {
		select {
		case id := <-dispatched:
			if firstOfPair == "" && (id == "low" || id == "high") {
				firstOfPair = id
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for the low/high pair to dispatch (got firstOfPair=%q so far)", firstOfPair)
		}
	}
	if firstOfPair != "high" {
		t.Errorf("first of {low,high} dispatched = %q, want %q (higher Priority must be preferred among disjoint candidates)", firstOfPair, "high")
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("RunPool did not return")
	}
}

// scopeID reads a pool spec's todo id; poolSpec always scopes exactly one todo.
func scopeID(spec CycleSpec) string {
	if len(spec.Scope) == 0 {
		return ""
	}
	return spec.Scope[0]
}
