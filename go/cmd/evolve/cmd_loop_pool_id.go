package main

import (
	"fmt"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/loopwave"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func (b *loopBatchCoordinator) mintPoolID(fc policy.FleetConfig) int {
	if !loopwave.ShouldRunPlan(fc) {
		return 0
	}
	updater, ok := b.deps.Storage.(core.StateUpdater)
	if !ok {
		fmt.Fprintln(b.stderr, "[loop] WARN: fleet: the state store cannot allocate a pool id; the lanes of this pool run without one")
		return 0
	}
	pool, err := core.AllocateCycleNumber(b.ctx, updater)
	if err != nil {
		fmt.Fprintf(b.stderr, "[loop] WARN: fleet: pool id not allocated (%v); the lanes of this pool run without one\n", err)
		return 0
	}
	return pool
}

func (b *loopBatchCoordinator) cycleLaunch(bin string, pool int) cycleLaunch {
	return cycleLaunch{Bin: bin, Root: b.cfg.ProjectRoot, GoalHash: b.cfg.GoalHash, GoalText: b.cfg.GoalText, Stdout: b.stdout, Stderr: b.stderr, Pool: pool}
}

func cycleRunPoolArgs(l cycleLaunch, outputContract string) []string {
	args := cycleRunArgs(l.GoalHash, outputContract, l.GoalText, l.Simulate, l.Root)
	if l.Pool > 0 {
		args = append(args, "--pool", strconv.Itoa(l.Pool))
	}
	return args
}
