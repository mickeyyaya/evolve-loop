package fleet

import (
	"context"
	"runtime"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

// PoolConfig configures a rolling lane pool (cycle-550 supervisor-continuous-
// lane-keeping). Target is the width the pool tries to hold — up to Target lanes
// run at once, drawn incrementally from the backlog. Concurrency optionally caps
// the live-lane count below Target (<=0 ⇒ follow Target); it never raises the cap
// above Target.
type PoolConfig struct {
	Target      int
	Concurrency int
}

// PoolTransition is the data backing the caller-formatted "lanes live: N/target"
// telemetry line, emitted on every live-lane-count change. This package stays
// I/O-free (same idiom as FleetConfig.Warnings) — the caller formats/logs it.
type PoolTransition struct {
	Live   int
	Target int
}

// RunPool maintains up to cfg.Target concurrently-running lanes drawn from
// backlog and BACKFILLS a replacement the instant any lane exits (PASS or FAIL),
// instead of the wave barrier's "wait for every sibling before re-planning". It
// removes the min-over-time width collapse Supervisor.Run suffers when lane
// durations are skewed (batch bh1rt946t: one early exit stranded the wave 1-wide).
//
// Selection: the initial fill dispatches disjoint todos in backlog order (the
// SAME file-ownership rule Partition applies statically, here applied
// INCREMENTALLY against the currently-RUNNING set). On any lane exit it selects
// the highest-Priority pending todo whose Files are disjoint from every
// STILL-RUNNING lane's Files (lowest backlog index breaks ties) and dispatches it
// as a replacement, without waiting for any sibling. When no disjoint candidate
// exists the pool simply runs fewer lanes — never zero while pending work
// remains, never the unisolated in-supervisor sequential path (per L4, every
// dispatch goes through the same isolated `launch` seam as Supervisor.Run).
//
// onTransition (nil-safe) fires on every live-lane-count change. Result.Index
// indexes into backlog. A zero-length backlog returns immediately with zero
// results and zero launch calls (the pool-mode analogue of the wave path's
// empty-plan guard). RunPool returns once every backlog item has been dispatched
// and has finished.
func RunPool(ctx context.Context, cfg PoolConfig, backlog []Todo, launch LaunchFn, onTransition func(PoolTransition)) []Result {
	results := make([]Result, len(backlog))
	if len(backlog) == 0 || launch == nil {
		return results
	}

	limit := cfg.Target
	if cfg.Concurrency > 0 && cfg.Concurrency < limit {
		limit = cfg.Concurrency
	}
	if limit < 1 {
		limit = 1 // never a zero-width pool while backlog remains
	}

	pending := make(map[int]bool, len(backlog))
	for i := range backlog {
		pending[i] = true
	}
	claimed := map[string]bool{} // normalized file → held by a running lane
	running := 0
	completions := make(chan int, len(backlog))

	emit := func() {
		if onTransition != nil {
			onTransition(PoolTransition{Live: running, Target: cfg.Target})
		}
	}
	disjoint := func(idx int) bool {
		for f := range normalizeFiles(backlog[idx].Files) {
			if claimed[f] {
				return false
			}
		}
		return true
	}
	dispatch := func(idx int) {
		delete(pending, idx)
		for f := range normalizeFiles(backlog[idx].Files) {
			claimed[f] = true
		}
		running++
		emit()
		// started rendezvous: the lane goroutine sends BEFORE calling launch, and
		// dispatch blocks until it does, so the lane is guaranteed scheduled (and
		// runs on into launch) before the pool considers this slot filled or
		// dispatches any later lane. Without it the Go scheduler's LIFO runnext
		// runs the FIRST-dispatched lane LAST, so a backfilled replacement could
		// be observed before an earlier still-running lane — the pool's realized
		// dispatch order must follow its decision order.
		started := make(chan struct{})
		go func() {
			started <- struct{}{}
			code, err := launch(ctx, poolSpec(backlog[idx], limit))
			results[idx] = Result{Index: idx, ExitCode: code, Err: err}
			completions <- idx
		}()
		<-started
		// Hand the P to the just-started lane so it runs its launch body to its own
		// block/return point before the next dispatch is decided — realized dispatch
		// order then follows decision order under cooperative (single-P) scheduling.
		runtime.Gosched()
	}

	// Initial fill: dispatch disjoint todos in backlog order up to the cap.
	for i := 0; i < len(backlog) && running < limit; i++ {
		if pending[i] && disjoint(i) {
			dispatch(i)
		}
	}

	// Roll: on each lane exit, free its files and backfill by highest Priority.
	for running > 0 {
		idx := <-completions
		for f := range normalizeFiles(backlog[idx].Files) {
			delete(claimed, f)
		}
		running--
		emit()
		for running < limit {
			cand := selectDisjoint(backlog, pending, disjoint)
			if cand < 0 {
				break
			}
			dispatch(cand)
		}
	}
	return results
}

// selectDisjoint returns the highest-Priority pending todo whose files are
// disjoint from every running lane (lowest backlog index breaks ties), or -1 when
// no pending todo can be co-scheduled with the current running set.
func selectDisjoint(backlog []Todo, pending map[int]bool, disjoint func(int) bool) int {
	best := -1
	for i := range backlog {
		if !pending[i] || !disjoint(i) {
			continue
		}
		if best < 0 || backlog[i].Priority > backlog[best].Priority {
			best = i
		}
	}
	return best
}

// poolSpec builds the isolated launch spec for one backlog todo. It mirrors
// PlanCycles' single-todo scope (Scope + Env[FleetScopeKey]) and forces
// EVOLVE_FLEET on exactly as Supervisor.launchOne does, so a pool dispatch is the
// SAME isolated seam as the wave path — never the unisolated sequential fallback.
func poolSpec(td Todo, width int) CycleSpec {
	env := map[string]string{
		ipcenv.FleetScopeKey: td.ID,
		ipcenv.FleetKey:      "1",
	}
	if width > 0 {
		env[ipcenv.FleetWidthKey] = strconv.Itoa(width)
	}
	return CycleSpec{
		Scope:          []string{td.ID},
		OutputContract: td.OutputContract,
		Optional:       td.Optional,
		Env:            env,
	}
}
