package main

import (
	"context"
	"fmt"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// poolPlanFn produces the rolling pool's backlog of file-disjoint todos to roll
// through: the pool analogue of wavePlanFn's decisionJSON+cardPackages.
type poolPlanFn func(ctx context.Context, waveIndex int) ([]fleet.Todo, error)

// shouldRunPool and shouldRunWave are mutually exclusive: a default/"wave"
// fleet never enters the pool, and a "pool" fleet never enters the wave
// barrier, so wiring the pool branch cannot double-dispatch nor regress the
// wave path.
func shouldRunPool(fc policy.FleetConfig) bool {
	return fc.Count > 1 && fc.PlanSource == "triage" && fc.Scheduling == "pool"
}

// dispatchPoolIteration reports ran=false with no side effects when
// shouldRunPool gates the pool path off, so the caller falls through
// unchanged. It reuses the same dirty-control-plane preflight guard and the
// same isolated launch seam the wave path's Supervisor uses.
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
		// Reports ran=false rather than burning a --max-cycles iteration on an
		// empty pool.
		return false, nil, nil, nil
	}
	results = fleet.RunPool(ctx, fleet.PoolConfig{Target: fc.Count, Concurrency: fc.Concurrency}, backlog, launch, nil)
	return true, backlog, results, nil
}

// productionPoolPlanFn reuses the wave path's plan source (productionWavePlanFn)
// and the same fleet.TodosFromTriage parse fleet.PlanFromTriage uses, so the
// pool and wave schedulers read identical committed work.
func productionPoolPlanFn(cfg loopConfig, storage core.Storage, count int, stderr io.Writer) poolPlanFn {
	wavePlan := productionWavePlanFn(cfg, storage, count, stderr)
	return func(ctx context.Context, waveIndex int) ([]fleet.Todo, error) {
		decisionJSON, cardPackages, err := wavePlan(ctx, waveIndex)
		if err != nil {
			return nil, err
		}
		// Same resolver as the wave path, freshly built per plan call; refusals
		// WARN inside the resolver wrapper.
		// See ADR-0074.
		todos, _, err := fleet.TodosFromTriage(decisionJSON, cardPackages, consoleRoutedResolver(cfg.ProjectRoot, stderr))
		return todos, err
	}
}
