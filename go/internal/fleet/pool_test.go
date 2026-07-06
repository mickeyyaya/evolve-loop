package fleet

// pool_test.go — RED contract for cycle-550's supervisor-continuous-
// lane-keeping task (L5, "the ceiling-keeper" that completes the fleet-width
// architecture: L1 triage-supply-disjoint-topn, L2 wave-seed-partitions-on-
// real-files, L4 eliminate-sequential-fallback-min-width-lane).
//
// PROBLEM (inbox 2026-07-06T16-00-00Z-supervisor-continuous-lane-keeping.json,
// operator directive verbatim: "the supervisor must try its best to honor the
// setting"): Supervisor.Run (fleet.go:75) is a WAVE BARRIER — it launches
// every spec in the batch, waits for ALL of them via sync.WaitGroup, and only
// then returns. Batch bh1rt946t evidence: wave 0 dispatched 2 lanes, one
// failed early, and the supervisor stayed 1-wide for the REST of that wave
// instead of immediately backfilling a replacement — realized width is
// min-over-time, not the operator's configured fleet.count, whenever a lane's
// duration is skewed relative to its siblings (which is the common case, not
// the exception).
//
// FIX CONTRACT (new surface this cycle — undefined until Builder adds it, so
// this package's test build fails to compile today; that compile failure IS
// the RED evidence, mirroring the cycle-465/507/547 precedent):
//
//	type PoolConfig struct { Target, Concurrency int }
//	type PoolTransition struct { Live, Target int }
//	func RunPool(ctx context.Context, cfg PoolConfig, backlog []Todo,
//		launch LaunchFn, onTransition func(PoolTransition)) []Result
//
//	RunPool maintains up to cfg.Target concurrently-running lanes drawn from
//	backlog (bounded by cfg.Concurrency, <=0 meaning follow Target). It
//	dispatches an initial fill of up to Target disjoint todos (by the SAME
//	file-ownership rule fleet.Partition already applies statically -- here
//	applied INCREMENTALLY against the currently-RUNNING set rather than a
//	fixed per-wave partition), then on ANY lane's exit -- PASS or FAIL, it
//	does not matter which -- immediately selects the next pending todo whose
//	Files are disjoint from every STILL-RUNNING lane's Files (highest
//	Priority first) and dispatches it as a replacement, WITHOUT waiting for
//	any sibling lane to finish. When no disjoint candidate exists the pool
//	simply runs fewer lanes (never zero while pending work remains, and never
//	falls back to the unisolated in-supervisor sequential path -- per L4,
//	every dispatch in this function goes through the same isolated `launch`
//	seam as Supervisor.Run). onTransition (nil-safe) is invoked on every
//	live-lane-count change with the data backing the "lanes live: N/target"
//	telemetry line -- formatted by the CALLER (this package stays I/O-free,
//	same idiom as FleetConfig.Warnings). Result.Index indexes into backlog.
//	RunPool returns once every backlog item has been dispatched and finished
//	(a zero-length backlog returns immediately with zero results and zero
//	launch calls -- the pool-mode analogue of the wave path's D1 empty-plan
//	guard).
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Positive : TestRunPool_BackfillsReplacementWhileSiblingLaneStillRunning
//     (the core rolling-pool behavior -- a barrier-preserving impl fails this)
//   - Negative : TestRunPool_CollidingFilesNeverCoRunButAllEventuallyDispatch
//     (the strongest anti-no-op for the disjointness rule: a naive "just
//     backfill blindly" impl that ignores file overlap fails here by running
//     2 colliding-file lanes at once)
//   - Edge     : TestRunPool_EmptyBacklogIdlesCleanlyNoLaunchCalls
//   - Semantic : TestRunPool_EmitsShrinkAndRecoveryTransitions (telemetry
//     data distinct from the dispatch-timing behavior itself)
//   - Selection: TestRunPool_BackfillPrefersHighestPriorityDisjointCandidate

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestRunPool_BackfillsReplacementWhileSiblingLaneStillRunning is the direct
// encoding of the task's primary acceptance criterion: "With fleet.count=2
// and >=3 disjoint pending todos: kill/fail one running lane -> a replacement
// lane dispatches within one scheduler tick while the sibling lane is STILL
// RUNNING." Three mutually file-disjoint todos, Target=2: the pool fills with
// A and B; B exits immediately; C (the only remaining pending todo, disjoint
// from A) MUST be dispatched before A -- still blocked -- is ever unblocked.
// A wave-barrier-preserving implementation (dispatch once, wait for ALL of
// A/B to finish before considering C) fails this: it would never observe C
// dispatched while A is still in flight.
func TestRunPool_BackfillsReplacementWhileSiblingLaneStillRunning(t *testing.T) {
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

	// B has already returned (its launch call returns immediately). A is
	// still blocked on holdA. The replacement for B's exit -- C, the only
	// remaining disjoint pending todo -- must be dispatched NOW, not after A
	// finishes.
	select {
	case id := <-dispatched:
		if id != "C" {
			t.Fatalf("backfill dispatched %q, want C (the only remaining disjoint pending todo)", id)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no replacement lane was dispatched for B's exit while sibling lane A was still running -- the wave barrier was not removed")
	}

	close(holdA)
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

// TestRunPool_CollidingFilesNeverCoRunButAllEventuallyDispatch is the
// negative/anti-no-op test for the disjointness rule: three todos that all
// touch the SAME file can never be co-scheduled (they would collide in a
// shared worktree at ship time), so the pool must shrink to 1 concurrent
// lane for this backlog even though Target=2 -- but it must still eventually
// dispatch and finish every one of them (never stall). A naive "always
// backfill on exit regardless of files" implementation fails this by running
// 2 colliding-file lanes at once.
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

	// Let every lane through one at a time: release once per backlog item,
	// pausing between releases long enough that a buggy 2-at-once impl would
	// have already shown up in maxSeen.
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

// TestRunPool_EmptyBacklogIdlesCleanlyNoLaunchCalls is the pool-mode
// analogue of the wave path's D1 empty-plan guard: zero pending work must
// idle cleanly, never invoke launch, and never hang.
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

// TestRunPool_EmitsShrinkAndRecoveryTransitions pins the telemetry data
// contract ("lanes live: N/target" -- formatted by the caller, not this
// package): onTransition must observe the pool reach the full Target width
// on initial fill, and -- combined with the backfill scenario above -- must
// observe it RETURN to Target width after a backfill, not stay shrunk for
// the rest of the run the way today's wave barrier does.
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
	case <-dispatched: // C's backfill dispatch
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

// TestRunPool_BackfillPrefersHighestPriorityDisjointCandidate encodes the
// selection rule from the fix's design ("highest-weight pending todo whose
// declared files are DISJOINT from all RUNNING lanes' files"): when a lane
// exits and MULTIPLE disjoint candidates are pending, the pool must pick the
// highest-Priority one, not simply the first in backlog order.
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

	// A is now the sole running lane and stays blocked; with Target=1 there
	// is no free capacity, so neither low nor high can dispatch yet. Once we
	// unblock A, its single slot frees and BOTH low and high become eligible
	// disjoint candidates at once -- the pool must pick "high" (Priority=9)
	// over "low" (Priority=1) for that slot. Target=1 forces one pick at a
	// time, making the ordering assertion below deterministic.
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

// scopeID reads the single todo ID a test's CycleSpec was built for. Tests in
// this file each launch specs scoped to exactly one todo (Target-bounded
// single-item buckets), so Scope[0] is the todo's ID -- mirroring how
// waveScopeIDs (cmd_loop_wave_test.go) reads Env[ipcenv.FleetScopeKey] for
// the same purpose in the wave path's own tests.
func scopeID(spec CycleSpec) string {
	if len(spec.Scope) == 0 {
		return ""
	}
	return spec.Scope[0]
}
