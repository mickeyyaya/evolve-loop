package main

import (
	"context"
	"fmt"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type poolPlanFn func(ctx context.Context, waveIndex int) ([]fleet.Todo, error)

func shouldRunPool(fc policy.FleetConfig) bool {
	return fc.Count > 1 && fc.PlanSource == "triage" && fc.Scheduling == "pool"
}

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
		return false, nil, nil, nil
	}
	results = fleet.RunPool(ctx, fleet.PoolConfig{Target: fc.Count, Concurrency: fc.Concurrency}, backlog, launch, nil)
	return true, backlog, results, nil
}

func productionPoolPlanFn(cfg loopConfig, storage core.Storage, count int, stderr io.Writer) poolPlanFn {
	wavePlan := productionWavePlanFn(cfg, storage, count, stderr)
	return func(ctx context.Context, waveIndex int) ([]fleet.Todo, error) {
		decisionJSON, cardPackages, err := wavePlan(ctx, waveIndex)
		if err != nil {
			return nil, err
		}
		todos, _, err := fleet.TodosFromTriage(decisionJSON, cardPackages, consoleRoutedResolver(cfg, stderr))
		return todos, err
	}
}
