package fleet

import (
	"context"
	"runtime"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

// PoolConfig sets the width a rolling pool holds; a positive Concurrency may only lower it below Target.
type PoolConfig struct {
	Target      int
	Concurrency int
}

// PoolTransition is the live-lane count the caller formats as "lanes live: N/target".
type PoolTransition struct {
	Live   int
	Target int
}

// RunPool keeps up to cfg.Target file-disjoint lanes running, backfilling as each exits; Result.Index indexes backlog.
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
		limit = 1
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
		// Correctness comes from claiming files and counting the lane before its goroutine
		// starts; the rendezvous and Gosched are hints and do not order launch callbacks.
		started := make(chan struct{})
		go func() {
			started <- struct{}{}
			code, err := launch(ctx, poolSpec(backlog[idx], limit))
			results[idx] = Result{Index: idx, ExitCode: code, Err: err}
			completions <- idx
		}()
		<-started
		runtime.Gosched()
	}

	for i := 0; i < len(backlog) && running < limit; i++ {
		if pending[i] && disjoint(i) {
			dispatch(i)
		}
	}

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

// selectDisjoint returns the highest-Priority disjoint pending todo (lowest index on ties), or -1.
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

// poolSpec builds the same scoped fleet-mode spec a wave launch gets, so a pool lane is equally isolated.
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
