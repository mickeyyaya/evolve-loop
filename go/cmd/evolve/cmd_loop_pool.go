package main

import (
	"context"
	"fmt"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/loopwave"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type poolPlanFn func(ctx context.Context, waveIndex int) ([]fleet.Todo, error)

type poolPlanAt func(depth int) poolPlanFn

type poolSupplyFn func(held []fleet.Todo) ([]fleet.Todo, error)

type poolRunFn func(ctx context.Context, backlog []fleet.Todo, resupply poolSupplyFn) []fleet.Result

func rollingPool(fc policy.FleetConfig, launch fleet.LaunchFn) poolRunFn {
	return func(ctx context.Context, backlog []fleet.Todo, _ poolSupplyFn) []fleet.Result {
		return fleet.RunPool(ctx, fleet.PoolConfig{Target: fc.Count, Concurrency: fc.Concurrency}, backlog, launch, nil)
	}
}

func dispatchPoolIteration(ctx context.Context, fc policy.FleetConfig, preflight func() error, planFn poolPlanFn, launch fleet.LaunchFn, waveIndex int) (ran bool, results []fleet.Result, err error) {
	return planAndRunPool(ctx, fc, preflight, func(int) poolPlanFn { return planFn }, rollingPool(fc, launch), waveIndex)
}

func planAndRunPool(ctx context.Context, fc policy.FleetConfig, preflight func() error, plan poolPlanAt, run poolRunFn, waveIndex int) (ran bool, results []fleet.Result, err error) {
	if !loopwave.ShouldRunPool(fc) {
		return false, nil, nil
	}
	if err := preflight(); err != nil {
		return false, nil, fmt.Errorf("pool %d: control-plane preflight: %w", waveIndex, err)
	}
	depth := loopwave.PoolSupply(fc)
	backlog, err := plan(depth)(ctx, waveIndex)
	if err != nil {
		return false, nil, fmt.Errorf("pool %d: backlog plan: %w", waveIndex, err)
	}
	if len(backlog) == 0 {
		return false, nil, nil
	}
	resupply := func(held []fleet.Todo) ([]fleet.Todo, error) { return plan(depth+len(held))(ctx, waveIndex) }
	return true, run(ctx, backlog, resupply), nil
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
