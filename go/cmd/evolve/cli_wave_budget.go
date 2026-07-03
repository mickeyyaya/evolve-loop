package main

// cli_wave_budget.go is the production assembly for Q4 quota-driven wave sizing:
// it measures each family's live quota (reusing the proactive usage-probe's
// per-family bridge controller) and the pipeline's recent pace (budgethistory),
// hands both to the pure quotaAwareWaveConfig sizer, and idles the loop for the
// budget-computed inter-wave PaceDelay. Every measurement is fail-open and runs
// ONLY when the operator supplied a fleet.budget block — no block ⇒ no probe,
// no added latency, byte-identical lanes.

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

// budgetAwareWaveConfig resolves this wave's lane count + inter-wave pace. It
// always applies the quota-bench shrink; when a fleet.budget block is present it
// additionally probes live quota + recent pace and sizes via fleetbudget.Plan
// (shadow logs the decision, enforce applies it). A single now is shared by the
// quota snapshot and the plan so ObservedAt and the reset-horizon math agree.
// The loop's ctx threads all the way into the probe + storage read so a
// SIGINT/SIGTERM during measurement cancels it (mirroring productionWavePlanFn).
func budgetAwareWaveConfig(ctx context.Context, fleetCfg policy.FleetConfig, projectRoot, evolveDir string, storage core.Storage, stderr io.Writer) (policy.FleetConfig, time.Duration) {
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

// probeWaveQuota measures each installed interactive family's quota for the
// budget allocator, reusing the SAME per-family bridge controller assembly the
// proactive usage probe uses — one way to reach a family's /usage command. It
// captures each pane and parses it into a quotastate.QuotaState; individual
// family failures are omitted by usageprobe.ProbeQuota (fail-open).
func probeWaveQuota(ctx context.Context, projectRoot, evolveDir string, now time.Time, stderr io.Writer) []quotastate.QuotaState {
	families := bridge.InteractiveFamilies()
	if len(families) == 0 {
		return nil
	}
	factory := bridge.NewControllerFactory(projectRoot, filepath.Join(evolveDir, "budget-probe"), "budget-probe", bridge.Deps{})
	fmt.Fprintf(stderr, "[budget] probing %v for quota before wave sizing\n", families)
	return usageprobe.ProbeQuota(ctx, families, bridgeUsageProbe(factory), now)
}

// collectWaveThroughput rolls up the pipeline's recent pace: the last `window`
// completed cycles' durations via budgethistory.Collect. A non-positive window
// or no prior cycle yields the zero Throughput, which the allocator degrades to
// reset-pace / floor.
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

// paceBeforeNextWave idles the loop for the budget-computed inter-wave delay so
// an enforce-mode floor-forced wave doesn't burn quota before its reset. Zero
// delay (the default — shadow, or no floor pressure) returns immediately. The
// idle is interruptible: a SIGINT/SIGTERM during the pause cancels ctx and the
// loop stops promptly instead of sleeping out the full delay.
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
