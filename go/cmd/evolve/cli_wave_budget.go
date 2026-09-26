package main

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/budgethistory"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
	"github.com/mickeyyaya/evolve-loop/go/internal/usageprobe"
)

// budgetAwareWaveConfig resolves this wave's lane count and inter-wave pace;
// quota and pace are measured only when a fleet.budget block is present.
func budgetAwareWaveConfig(ctx context.Context, fleetCfg policy.FleetConfig, projectRoot, evolveDir string, storage core.Storage, stderr io.Writer) (policy.FleetConfig, time.Duration) {
	// One now for the quota snapshot and the plan keeps ObservedAt and the
	// reset-horizon math in agreement.
	now := time.Now()
	var (
		states []quotastate.QuotaState
		tp     budgethistory.Throughput
	)
	if fleetCfg.Budget != nil {
		states = probeWaveQuota(ctx, projectRoot, evolveDir, now, stderr)
		tp = collectWaveThroughput(ctx, projectRoot, storage, fleetCfg.Budget.HistoryWindow)
	}
	return quotaAwareWaveConfig(fleetCfg, projectRoot, stderr, states, tp, now)
}

// probeWaveQuota measures each installed family's quota through the usage
// probe's bridge path; a family that fails to answer is omitted.
func probeWaveQuota(ctx context.Context, projectRoot, evolveDir string, now time.Time, stderr io.Writer) []quotastate.QuotaState {
	families := bridge.InteractiveFamilies()
	if len(families) == 0 {
		return nil
	}
	factory := bridge.NewControllerFactory(projectRoot, filepath.Join(evolveDir, "budget-probe"), "budget-probe", bridge.Deps{})
	fmt.Fprintf(stderr, "[budget] probing %v for quota before wave sizing\n", families)
	return usageprobe.ProbeQuota(ctx, families, bridgeUsageProbe(factory), now)
}

// collectWaveThroughput rolls up the last window cycles' pace; the zero
// Throughput it returns without history makes the allocator fall back to the floor.
func collectWaveThroughput(ctx context.Context, projectRoot string, storage core.Storage, window int) budgethistory.Throughput {
	if window <= 0 {
		return budgethistory.Throughput{}
	}
	last, err := readLastCycleNumber(ctx, storage)
	if err != nil || last <= 0 {
		return budgethistory.Throughput{}
	}
	cycles := make([]int, 0, window)
	for n := last; n > 0 && len(cycles) < window; n-- {
		cycles = append(cycles, n)
	}
	return budgethistory.Collect(projectRoot, cycles)
}

// paceBeforeNextWave idles for the budget's inter-wave delay, so an enforce-mode
// wave does not burn quota before its reset; an interrupt ends the idle.
func paceBeforeNextWave(ctx context.Context, delay time.Duration, stderr io.Writer) {
	if delay <= 0 {
		return
	}
	fmt.Fprintf(stderr, "[budget] pacing %s before next wave (enforce)\n", delay.Round(time.Second))
	t := time.NewTimer(delay)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
