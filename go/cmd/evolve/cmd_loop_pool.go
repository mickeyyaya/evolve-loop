// cmd_loop_pool.go — FLEET-AS-POLICY rolling-pool dispatch (cycle-553
// supervisor-continuous-lane-keeping). The pool analogue of cmd_loop_wave.go's
// wave-barrier dispatch: where dispatchIteration launches a wave and blocks on
// EVERY sibling before re-planning, dispatchPoolIteration drives the backlog
// through fleet.RunPool, which BACKFILLS a replacement lane the instant any lane
// exits — removing the min-over-time width collapse the wave barrier suffers on
// skewed lane durations (batch bh1rt946t). fleet.RunPool shipped cycle 550 with
// its own exhaustive pool_test.go but had ZERO call sites outside its package,
// and policy.fleet.scheduling=="pool" parsed into a knob wired to nothing; this
// file is that wiring. Gated behind Scheduling=="pool" (shouldRunPool), mutually
// exclusive with the wave path.
package main

import (
	"context"
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// poolPlanFn produces the rolling pool's backlog of file-disjoint todos to roll
// through (the pool analogue of wavePlanFn's decisionJSON+cardPackages). ctx
// threads loop shutdown into the plan read; waveIndex lets production wiring pick
// per-iteration state.
type poolPlanFn func(ctx context.Context, waveIndex int) ([]fleet.Todo, error)

// shouldRunPool gates the rolling-pool dispatch path. It requires the SAME fleet
// preconditions as shouldRunWave (Count>1 && PlanSource=="triage") PLUS the
// resolved Scheduling strategy being "pool". Mutually exclusive with
// shouldRunWave (which excludes Scheduling=="pool"): a default/"wave" fleet never
// enters the pool, and a "pool" fleet never enters the wave barrier — so wiring
// the pool branch cannot double-dispatch nor regress the shipped wave path.
func shouldRunPool(fc policy.FleetConfig) bool {
	return fc.Count > 1 && fc.PlanSource == "triage" && fc.Scheduling == "pool"
}

// dispatchPoolIteration runs one iteration's rolling-pool path when shouldRunPool
// gates it on, and reports ran=false (no side effects) otherwise so the caller
// falls through unchanged. On the pool path, in order: runs preflight (the SAME
// S3 dirty-control-plane guard dispatchIteration uses — a refusal surfaces
// wrapped/errors.Is-matchable with ran=false and NEITHER planFn NOR launch
// invoked); obtains the backlog via planFn (ctx threaded); and drives it through
// fleet.RunPool with the injected launch (the SAME isolated launch seam the wave
// path's Supervisor uses — per L4 no dispatch ever takes the unisolated
// in-process sequential path), sizing PoolConfig{Target:fc.Count,
// Concurrency:fc.Concurrency}. An empty backlog reports ran=false, err=nil so the
// caller falls back instead of consuming a --max-cycles iteration doing no work
// (the pool analogue of dispatchIteration's D1 empty-plan guard). Returns the
// backlog and the per-lane RunPool results.
func dispatchPoolIteration(ctx context.Context, fc policy.FleetConfig, preflight func() error, planFn poolPlanFn, launch fleet.LaunchFn, waveIndex int) (ran bool, backlog []fleet.Todo, results []fleet.Result, err error) {
	if !shouldRunPool(fc) {
		return false, nil, nil, nil
	}
	if err := preflight(); err != nil {
		return false, nil, nil, fmt.Errorf("pool %d: control-plane preflight: %w", waveIndex, err)
	}
	backlog, err = planFn(ctx, waveIndex)
	if err != nil {
		return false, nil, nil, fmt.Errorf("pool %d: backlog plan: %w", waveIndex, err)
	}
	if len(backlog) == 0 {
		// D1 empty-plan guard: never burn a --max-cycles iteration on an empty
		// pool — report ran=false so the caller falls back.
		return false, nil, nil, nil
	}
	results = fleet.RunPool(ctx, fleet.PoolConfig{Target: fc.Count, Concurrency: fc.Concurrency}, backlog, launch, nil)
	return true, backlog, results, nil
}

// productionPoolPlanFn builds the real poolPlanFn: it reuses the wave path's
// single-writer plan source (productionWavePlanFn — the prior cycle's triage
// decision, else the inbox seed) and adapts its decision bytes into the
// disjoint-aware fleet.Todo backlog RunPool rolls through, via the single-sourced
// fleet.TodosFromTriage (the SAME parse fleet.PlanFromTriage uses to partition
// the wave path). Single-sourcing the decision→todos parse keeps the pool and
// wave schedulers reading identical committed work.
func productionPoolPlanFn(cfg loopConfig, storage core.Storage, count int) poolPlanFn {
	wavePlan := productionWavePlanFn(cfg, storage, count)
	return func(ctx context.Context, waveIndex int) ([]fleet.Todo, error) {
		decisionJSON, cardPackages, err := wavePlan(ctx, waveIndex)
		if err != nil {
			return nil, err
		}
		return fleet.TodosFromTriage(decisionJSON, cardPackages)
	}
}
