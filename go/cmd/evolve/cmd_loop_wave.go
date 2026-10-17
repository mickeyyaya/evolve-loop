package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/budgethistory"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/loopwave"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

type (
	waveLauncher = loopwave.Launcher
	wavePlanFn   = loopwave.PlanFn
)

func newWaveEngine(cfg loopConfig, storage core.Storage, warn io.Writer, signals func() *signalcenter.Center) *loopwave.Engine {
	roots := loopwave.Roots{ProjectRoot: cfg.ProjectRoot, EvolveDir: cfg.EvolveDir}
	ports := loopwave.Ports{
		LastCycle: func(ctx context.Context) (int, error) { return readLastCycleNumber(ctx, storage) },
		Workspace: func(cycle int) string { return cycleWorkspace(cfg.ProjectRoot, cycle) },
		Protected: laneForbidden(cfg.ProjectRoot, warn),
		Shrink:    fleet.QuotaAwareCount,
		Chains:    cliroute.FamilyChains,
	}
	return loopwave.New(roots, ports, warn, loopwave.WithSignals(signals), loopwave.WithGoal(cfg.GoalHash))
}

func (b *loopBatchCoordinator) wave() *loopwave.Engine {
	if b.waveEngine == nil {
		b.waveEngine = b.wiredWave()
	}
	return b.waveEngine
}

func (b *loopBatchCoordinator) wiredWave() *loopwave.Engine {
	return newWaveEngine(b.cfg, b.deps.Storage, b.stderr, func() *signalcenter.Center { return b.deps.Signals })
}

func nilSignals() *signalcenter.Center { return nil }

func nullWaveEngine(roots loopwave.Roots, warn io.Writer) *loopwave.Engine {
	return newWaveEngine(loopConfig{ProjectRoot: roots.ProjectRoot, EvolveDir: roots.EvolveDir}, nil, warn, nilSignals)
}

func shouldRunWave(fc policy.FleetConfig) bool { return loopwave.ShouldRunWave(fc) }

func loadFleetConfig(evolveDir string) policy.FleetConfig { return loopwave.LoadFleetConfig(evolveDir) }

func productionWavePreflight(projectRoot string) func() error { return loopwave.Preflight(projectRoot) }

func productionWavePlanFn(cfg loopConfig, storage core.Storage, count int, stderr io.Writer) wavePlanFn {
	return newWaveEngine(cfg, storage, stderr, nilSignals).PlanFn(count)
}

func consoleRoutedResolver(cfg loopConfig, stderr io.Writer) fleet.RoutedFn {
	return newWaveEngine(cfg, nil, stderr, nilSignals).RoutedResolver()
}

func quotaAwareWaveConfig(fc policy.FleetConfig, projectRoot string, warn io.Writer, states []quotastate.QuotaState, tp budgethistory.Throughput, now time.Time) (policy.FleetConfig, time.Duration) {
	routing := loopwave.LoadRouting(loopwave.RootsOf(projectRoot).EvolveDir, warn)
	return nullWaveEngine(loopwave.RootsOf(projectRoot), warn).Size(fc, routing, states, tp, now)
}

func seedWavePlanFromInbox(evolveDir string, count int) ([]byte, error) {
	return loopwave.SeedWavePlanFromInbox(evolveDir, count, laneForbidden(filepath.Dir(evolveDir), os.Stderr))
}

func widenNarrowDecision(data []byte, evolveDir string, count int) []byte {
	return loopwave.WidenNarrowDecision(data, evolveDir, count, laneForbidden(filepath.Dir(evolveDir), os.Stderr))
}

func dispatchIteration(ctx context.Context, fc policy.FleetConfig, preflight func() error, planFn wavePlanFn, launcher waveLauncher, routed fleet.RoutedFn, waveIndex int) (ran bool, specs []fleet.CycleSpec, results []fleet.Result, err error) {
	out, err := nullWaveEngine(loopwave.Roots{}, io.Discard).Dispatch(ctx, loopwave.DispatchRequest{Config: fc, Wave: waveIndex, Preflight: preflight, Plan: planFn, Launcher: launcher, Routed: routed})
	return out.Ran, out.Specs, out.Results, err
}

func forceOneLaneDispatch(ctx context.Context, preflight func() error, planFn wavePlanFn, launcher waveLauncher, routed fleet.RoutedFn, waveIndex int) (ran bool, specs []fleet.CycleSpec, results []fleet.Result, err error) {
	out, err := nullWaveEngine(loopwave.Roots{}, io.Discard).ForceOneLane(ctx, loopwave.DispatchRequest{Wave: waveIndex, Preflight: preflight, Plan: planFn, Launcher: launcher, Routed: routed})
	return out.Ran, out.Specs, out.Results, err
}

func minWidthRepair(ctx context.Context, fleetCfg, waveCfg policy.FleetConfig, preflight func() error, planFn wavePlanFn, launcher waveLauncher, routed fleet.RoutedFn, waveIndex int, stderr io.Writer, signals *signalcenter.Center) (handled bool) {
	e := newWaveEngine(loopConfig{}, nil, stderr, func() *signalcenter.Center { return signals })
	return e.RepairMinWidth(ctx, fleetCfg, waveCfg, loopwave.DispatchRequest{Config: waveCfg, Wave: waveIndex, Preflight: preflight, Plan: planFn, Launcher: launcher, Routed: routed})
}

func productionWaveLauncher(fc policy.FleetConfig, binPath, projectRoot, goalHash, goalText string, stdout, stderr io.Writer) waveLauncher {
	return nullWaveEngine(loopwave.RootsOf(projectRoot), stderr).Launcher(0, fc.Concurrency, execCycleLaunch(binPath, false, projectRoot, goalHash, goalText, stdout, stderr))
}

func reloadFleetConfigAtWaveBoundary(evolveDir string, prev policy.FleetConfig, warn io.Writer) policy.FleetConfig {
	return loopwave.ReloadFleetConfig(evolveDir, prev, warn)
}
