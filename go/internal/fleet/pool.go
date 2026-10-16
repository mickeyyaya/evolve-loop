package fleet

import (
	"context"
	"runtime"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

type PoolConfig struct {
	Target      int
	Concurrency int
}

type PoolTransition struct {
	Live   int
	Target int
}

func RunPool(ctx context.Context, cfg PoolConfig, backlog []Todo, launch LaunchFn, onTransition func(PoolTransition)) []Result {
	if launch == nil {
		return noLaunchResults(len(backlog))
	}
	results := make([]Result, len(backlog))

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
	claimed := map[string]bool{}
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

func noLaunchResults(n int) []Result {
	results := make([]Result, n)
	for i := range results {
		results[i] = Result{Index: i, ExitCode: -1, Err: errNoLaunch}
	}
	return results
}

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
