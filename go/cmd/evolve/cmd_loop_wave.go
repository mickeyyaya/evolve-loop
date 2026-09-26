// cmd_loop_wave.go — the unit-13 wave seam (ADR-0103). The wave engine —
// the sequential-vs-wave gate, the ONE dispatch body the fan-out and the
// min-width repair share, the fleet-config loaders, the freshness-gated
// launcher, the quota/budget sizing and the plan source — lives in
// internal/loopwave. This file keeps the engine's ONE wired construction
// (newWaveEngine), the coordinator's accessor pair (wave / wiredWave) and the
// Strangler Fig facades the pool scheduler, the budget wrapper and the
// by-name tests keep, so no production call site learned the unit exists.
package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/budgethistory"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/loopwave"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

// The leaf's port vocabulary under the names the test doubles implement.
type (
	waveLauncher = loopwave.Launcher
	wavePlanFn   = loopwave.PlanFn
)

// newWaveEngine is the ONE loopwave.New( site (TestWaveEngine_OneConstructionSite):
// the lane-routing predicate, the prior-cycle readers over the
// host's storage, and fleet.QuotaAwareCount as the bench shrink — the shrink
// IS wired into the live wave path (acs/cycle467), by code.
func newWaveEngine(cfg loopConfig, storage core.Storage, warn io.Writer, signals func() *signalcenter.Center) *loopwave.Engine {
	roots := loopwave.Roots{ProjectRoot: cfg.ProjectRoot, EvolveDir: cfg.EvolveDir}
	ports := loopwave.Ports{
		LastCycle: func(ctx context.Context) (int, error) { return readLastCycleNumber(ctx, storage) },
		Workspace: func(cycle int) string { return cycleWorkspace(cfg.ProjectRoot, cycle) },
		Protected: laneForbidden(cfg.ProjectRoot, warn),
		Shrink:    fleet.QuotaAwareCount,
	}
	return loopwave.New(roots, ports, warn, loopwave.WithSignals(signals))
}

// wave returns the coordinator's wave engine, lazily built and cached: the
// coordinator is assembled as a literal (cmd_loop.go and the loop tests), so
// there is no constructor to build it eagerly in. The cache is unsynchronized
// by design: one batch is driven by ONE goroutine (the lanes are
// subprocesses), so wave() is never called concurrently — a caller that
// parallelizes iteration prep must build the engine eagerly instead.
func (b *loopBatchCoordinator) wave() *loopwave.Engine {
	if b.waveEngine == nil {
		b.waveEngine = b.wiredWave()
	}
	return b.waveEngine
}

// wiredWave builds the engine over the batch's config, storage, console and
// Center — the Center through an accessor, read live.
func (b *loopBatchCoordinator) wiredWave() *loopwave.Engine {
	return newWaveEngine(b.cfg, b.deps.Storage, b.stderr, func() *signalcenter.Center { return b.deps.Signals })
}

// nilSignals is the Null-Center accessor of the facades below.
func nilSignals() *signalcenter.Center { return nil }

// nullWaveEngine builds a Null-Center engine over roots for the facades that
// carry no storage (the pool's resolver, the budget wrapper, the launcher):
// the plan port is never reached on their paths. roots is never half
// populated: loopwave.RootsOf(projectRoot) where a facade has a root (both
// halves derived the production way), the zero loopwave.Roots{} where it has
// none (the pure dispatch facades reach no root) —
// TestNullWaveEngine_EveryConstructionCarriesBothRootsOrNone.
func nullWaveEngine(roots loopwave.Roots, warn io.Writer) *loopwave.Engine {
	return newWaveEngine(loopConfig{ProjectRoot: roots.ProjectRoot, EvolveDir: roots.EvolveDir}, nil, warn, nilSignals)
}

// --- production facades: spellings kept for cmd_loop_batch.go, cmd_loop_window.go, cmd_loop_pool.go, cli_wave_budget.go ---

// shouldRunWave is the ONE sequential-vs-wave decision (loopwave.ShouldRunWave).
func shouldRunWave(fc policy.FleetConfig) bool { return loopwave.ShouldRunWave(fc) }

// loadFleetConfig is the batch-start loader (defaults on any error).
func loadFleetConfig(evolveDir string) policy.FleetConfig { return loopwave.LoadFleetConfig(evolveDir) }

// productionWavePreflight is the S3 dirty-control-plane guard against the
// MAIN checkout (the pool shares it).
func productionWavePreflight(projectRoot string) func() error { return loopwave.Preflight(projectRoot) }

// productionWavePlanFn is the wave's plan source (the pool reuses it through
// productionPoolPlanFn — a Null Center there, so its prune line stays a line).
func productionWavePlanFn(cfg loopConfig, storage core.Storage, count int, stderr io.Writer) wavePlanFn {
	return newWaveEngine(cfg, storage, stderr, nilSignals).PlanFn(count)
}

// consoleRoutedResolver is the ADR-0074 plan-time gate's resolver (the pool
// scheduler reaches it with no Center, so its refusal WARN stays a line).
func consoleRoutedResolver(projectRoot string, stderr io.Writer) fleet.RoutedFn {
	return nullWaveEngine(loopwave.RootsOf(projectRoot), stderr).RoutedResolver()
}

// quotaAwareWaveConfig sizes fc for this wave: the bench shrink, then the
// budget block (cli_wave_budget.go's budgetAwareWaveConfig wraps it).
func quotaAwareWaveConfig(fc policy.FleetConfig, projectRoot string, warn io.Writer, states []quotastate.QuotaState, tp budgethistory.Throughput, now time.Time) (policy.FleetConfig, time.Duration) {
	return nullWaveEngine(loopwave.RootsOf(projectRoot), warn).Size(fc, states, tp, now)
}

// seedWavePlanFromInbox seeds a wave plan from the inbox backlog.
func seedWavePlanFromInbox(evolveDir string, count int) ([]byte, error) {
	return loopwave.SeedWavePlanFromInbox(evolveDir, count, laneForbidden(filepath.Dir(evolveDir), os.Stderr))
}

// widenNarrowDecision widens a narrow prior decision to fleet width.
func widenNarrowDecision(data []byte, evolveDir string, count int) []byte {
	return loopwave.WidenNarrowDecision(data, evolveDir, count, laneForbidden(filepath.Dir(evolveDir), os.Stderr))
}

// --- test/ACS-only facades: ZERO production callers (TestWaveEngine_OneConstructionSite); the coordinator drives the engine ---

// dispatchIteration is the by-name facade of Engine.Dispatch over a Null
// Center (its WARN is the coordinator's to render).
func dispatchIteration(ctx context.Context, fc policy.FleetConfig, preflight func() error, planFn wavePlanFn, launcher waveLauncher, routed fleet.RoutedFn, waveIndex int) (ran bool, specs []fleet.CycleSpec, results []fleet.Result, err error) {
	out, err := nullWaveEngine(loopwave.Roots{}, io.Discard).Dispatch(ctx, loopwave.DispatchRequest{Config: fc, Wave: waveIndex, Preflight: preflight, Plan: planFn, Launcher: launcher, Routed: routed})
	return out.Ran, out.Specs, out.Results, err
}

// forceOneLaneDispatch is the by-name facade of Engine.ForceOneLane.
func forceOneLaneDispatch(ctx context.Context, preflight func() error, planFn wavePlanFn, launcher waveLauncher, routed fleet.RoutedFn, waveIndex int) (ran bool, specs []fleet.CycleSpec, results []fleet.Result, err error) {
	out, err := nullWaveEngine(loopwave.Roots{}, io.Discard).ForceOneLane(ctx, loopwave.DispatchRequest{Wave: waveIndex, Preflight: preflight, Plan: planFn, Launcher: launcher, Routed: routed})
	return out.Ran, out.Specs, out.Results, err
}

// minWidthRepair is the by-name facade of Engine.RepairMinWidth over the
// caller's Center (the minwidth suites pass the production sink topology);
// rootless like the zero Roots — the request carries every collaborator.
func minWidthRepair(ctx context.Context, fleetCfg, waveCfg policy.FleetConfig, preflight func() error, planFn wavePlanFn, launcher waveLauncher, routed fleet.RoutedFn, waveIndex int, stderr io.Writer, signals *signalcenter.Center) (handled bool) {
	e := newWaveEngine(loopConfig{}, nil, stderr, func() *signalcenter.Center { return signals })
	return e.RepairMinWidth(ctx, fleetCfg, waveCfg, loopwave.DispatchRequest{Config: waveCfg, Wave: waveIndex, Preflight: preflight, Plan: planFn, Launcher: launcher, Routed: routed})
}

// productionWaveLauncher is the by-name facade of Engine.Launcher: a
// Supervisor over execCycleLaunch inside the freshness gate (wave 0: no
// caller carries a wave; the refill dir is <projectRoot>/.evolve as always).
func productionWaveLauncher(fc policy.FleetConfig, binPath, projectRoot, goalHash, goalText string, stdout, stderr io.Writer) waveLauncher {
	return nullWaveEngine(loopwave.RootsOf(projectRoot), stderr).Launcher(0, fc.Concurrency, execCycleLaunch(binPath, false, projectRoot, goalHash, goalText, stdout, stderr))
}

// reloadFleetConfigAtWaveBoundary is the by-name facade of
// loopwave.ReloadFleetConfig (its hold WARN renders on warn verbatim) — a
// package function like loadFleetConfig's, so no engine is built here.
func reloadFleetConfigAtWaveBoundary(evolveDir string, prev policy.FleetConfig, warn io.Writer) policy.FleetConfig {
	return loopwave.ReloadFleetConfig(evolveDir, prev, warn)
}
