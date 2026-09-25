package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/loopwave"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type batchFlow uint8

const (
	batchProceed batchFlow = iota
	batchNextIteration
	batchStopIterations
	batchReturn
)

type batchDecision struct {
	flow     batchFlow
	exitCode int
}

// prepareIteration refreshes every batch-wide input at the only safe boundary:
// after the previous window drained and before the next dispatch begins.
func (b *loopBatchCoordinator) prepareIteration(
	iteration int,
	fleetConfig *policy.FleetConfig,
	waveBinary *string,
	batchStartCycle int,
) batchDecision {
	if b.ctx.Err() != nil {
		return b.interruptReturn(iteration, "")
	}
	// Every iteration, sequential and fleet: a SIGKILLed lane's live-phase
	// residue is invisible to the once-per-batch unfinishedCycle guard.
	// Before the breaker, so even a halted batch leaves a coherent record.
	reconcileStaleCycleState(b.ctx, b.deps.Storage, b.cfg.ProjectRoot, time.Now(), b.stderr)
	if exitCode, halted := blockerBreakerHalt(b.cfg.EvolveDir, b.cfg.ProjectRoot, batchStartCycle, b.stderr, b.deps.Signals); halted {
		b.result.StopReason = "pipeline_blocker_halt"
		b.result.emitFatal(b.stdout, b.stderr, b.cfg, 0)
		return batchDecision{flow: batchReturn, exitCode: exitCode}
	}

	// An interrupt that landed during the probes must not dispatch a wave
	// that is cancelled at spawn (2026-09-15: "wave 2: 0/2 lanes ok" printed
	// after the boundary SIGINT).
	if err := runPreWaveProbes(b.ctx, b.cfg.ProjectRoot, b.cfg.EvolveDir, b.cycleEnv, b.stderr); err != nil {
		return b.interruptReturn(iteration, "during the pre-wave probes ")
	}
	if _, halt := syncMainFromOriginAtWaveBoundary(b.ctx, b.cfg.ProjectRoot, b.stderr); halt != nil {
		b.result.StopReason = "plane_diverged_halt"
		if errors.Is(halt, errMainCIRed) {
			b.result.StopReason = "main_ci_red_halt"
		}
		emitLoopHalt(b.deps.Signals, 0, "loopBatchCoordinator.prepareIteration", CodeLoopHalt, halt.Error(), map[string]string{"stop_reason": b.result.StopReason})
		b.result.emitFatal(b.stdout, b.stderr, b.cfg, 0)
		return batchDecision{flow: batchReturn, exitCode: 2}
	}

	*fleetConfig = loopwave.ReloadFleetConfig(b.cfg.EvolveDir, *fleetConfig, b.stderr)
	if maybeRefreshChainBoundaryWithSignals(b.cfg, iteration+1, b.stderr, b.deps.Signals) {
		b.result.StopReason = "loop_boundary_refresh_reexec"
		if entry, err := lastChainBoundaryRefreshLogEntry(b.cfg.EvolveDir); err == nil {
			b.result.BoundaryRefresh = entry
		}
		b.result.emit(b.stdout)
		return batchDecision{flow: batchReturn}
	}

	if (shouldRunWave(*fleetConfig) || shouldRunPool(*fleetConfig)) && *waveBinary == "" {
		binary, err := os.Executable()
		if err != nil {
			fmt.Fprintf(b.stderr, "[loop] WARN: fleet: cannot resolve binary for fleet dispatch, staying sequential: %v\n", err)
			fleetConfig.Count = 1
		} else {
			*waveBinary = binary
		}
	}
	return batchDecision{flow: batchProceed}
}

// interruptReturn reports a SIGINT/SIGTERM caught at one of prepareIteration's
// two check points ("" at entry, or "during the pre-wave probes " after them)
// and returns the batchDecision that stops the batch cleanly.
func (b *loopBatchCoordinator) interruptReturn(iteration int, when string) batchDecision {
	signalStop(b.stdout, b.stderr, b.result, fmt.Sprintf("%sbefore cycle %d — stopping", when, iteration+1))
	return batchDecision{flow: batchReturn, exitCode: 130}
}

// dispatchFleetIteration runs one rolling pool or barrier wave. A failed plan
// falls through to the sequential executor; a completed fleet window advances
// the batch without touching sequential-only breaker state.
func (b *loopBatchCoordinator) dispatchFleetIteration(
	iteration int,
	fleetConfig policy.FleetConfig,
	waveBinary string,
	starvation *fleet.StarvationTracker,
) batchDecision {
	if shouldRunPool(fleetConfig) {
		poolLaunch := execCycleLaunch(waveBinary, false, b.cfg.ProjectRoot, b.cfg.GoalHash, b.cfg.GoalText, b.stdout, b.stderr)
		ran, _, results, err := dispatchPoolIteration(
			b.ctx,
			fleetConfig,
			productionWavePreflight(b.cfg.ProjectRoot),
			productionPoolPlanFn(b.cfg, b.deps.Storage, fleetConfig.Count, b.stderr),
			poolLaunch,
			iteration,
		)
		switch {
		case err != nil:
			fmt.Fprintf(b.stderr, "[loop] WARN: fleet: pool %d dispatch failed, falling back to sequential: %v\n", iteration, err)
		case ran:
			fmt.Fprintf(b.stderr, "[loop] pool %d: %d/%d lanes ok (rolling, target=%d)\n",
				iteration, len(results)-failedLaneCount(results), len(results), fleetConfig.Count)
			if decision := b.fleetHaltDecision("pool", iteration, results); decision.flow == batchReturn {
				return decision
			}
			applyEscalationBoundary(b.cfg.EvolveDir, iteration, b.stderr, b.deps.Signals)
			return batchDecision{flow: batchNextIteration}
		default:
			fmt.Fprintf(b.stderr, "[loop] WARN: fleet: pool %d planned zero lanes (empty backlog), falling back to sequential\n", iteration)
		}
	}

	if !shouldRunWave(fleetConfig) {
		return batchDecision{flow: batchProceed}
	}
	waveConfig, pace := budgetAwareWaveConfig(b.ctx, fleetConfig, b.cfg.ProjectRoot, b.cfg.EvolveDir, b.deps.Storage, b.stderr)
	launcher := b.wave().Launcher(iteration, waveConfig.Concurrency, execCycleLaunch(waveBinary, false, b.cfg.ProjectRoot, b.cfg.GoalHash, b.cfg.GoalText, b.stdout, b.stderr))
	out, err := b.wave().Dispatch(b.ctx, loopwave.DispatchRequest{
		Config:    waveConfig,
		Wave:      iteration,
		Preflight: productionWavePreflight(b.cfg.ProjectRoot),
		Plan:      b.wave().PlanFn(waveConfig.Count),
		Launcher:  launcher,
		Routed:    b.wave().RoutedResolver(),
	})
	ran, results := out.Ran, out.Results
	switch {
	case err != nil:
		// The engine reported LOOP_WAVE_DISPATCH_FAILED (rendered by the root
		// sink); the batch falls through to the sequential body.
	case ran:
		fmt.Fprintf(b.stderr, "[loop] wave %d: %d/%d lanes ok\n", iteration, len(results)-failedLaneCount(results), len(results))
		emitLoopWave(b.deps.Signals, iteration, "loopBatchCoordinator.dispatchFleetIteration", "",
			fmt.Sprintf("wave %d: %d/%d lanes ok", iteration, len(results)-failedLaneCount(results), len(results)),
			map[string]string{"lanes_ok": strconv.Itoa(len(results) - failedLaneCount(results)), "lanes": strconv.Itoa(len(results))})
		if decision := b.fleetHaltDecision("wave", iteration, results); decision.flow == batchReturn {
			return decision
		}
		observation := fleet.WaveObservation{
			DesiredLanes:  fleetConfig.Count,
			RealizedLanes: len(results),
			QuotaShrunk:   waveConfig.Count < fleetConfig.Count,
		}
		if starvation.Observe(observation, fleetConfig.StarvationK) {
			item := fleet.BuildStarvationItem(observation, fleetConfig.StarvationK, fleetConfig.StarvationWeight, iteration, time.Now().UTC().Format(time.RFC3339))
			if path, err := item.WriteTo(b.cfg.EvolveDir); err != nil {
				fmt.Fprintf(b.stderr, "[loop] WARN: fleet: could not self-file starvation todo: %v\n", err)
			} else {
				fmt.Fprintf(b.stderr, "[loop] fleet: work-supply starvation after %d waves — self-filed %s\n", fleetConfig.StarvationK, path)
			}
		}
		applyEscalationBoundary(b.cfg.EvolveDir, iteration, b.stderr, b.deps.Signals)
		paceBeforeNextWave(b.ctx, pace, b.stderr)
		return batchDecision{flow: batchNextIteration}
	default:
		oneLauncher := b.wave().Launcher(iteration, fleetConfig.Concurrency, execCycleLaunch(waveBinary, false, b.cfg.ProjectRoot, b.cfg.GoalHash, b.cfg.GoalText, b.stdout, b.stderr))
		if b.wave().RepairMinWidth(b.ctx, fleetConfig, waveConfig, loopwave.DispatchRequest{
			Config:    fleetConfig,
			Wave:      iteration,
			Preflight: productionWavePreflight(b.cfg.ProjectRoot),
			Plan:      b.wave().PlanFn(fleetConfig.Count),
			Launcher:  oneLauncher,
			Routed:    b.wave().RoutedResolver(),
		}) {
			return batchDecision{flow: batchNextIteration}
		}
	}
	return batchDecision{flow: batchProceed}
}

func (b *loopBatchCoordinator) fleetHaltDecision(kind string, iteration int, results []fleet.Result) batchDecision {
	exitCode, stopReason, halt := dispatchHaltDecision(results)
	if !halt {
		return batchDecision{flow: batchProceed}
	}
	emitLoopHalt(b.deps.Signals, 0, "loopBatchCoordinator.fleetHaltDecision", CodeLoopFleetLaneHalt,
		fmt.Sprintf("a fleet lane in %s %d exited with the ADR-0072 halt code (rc=%d); the lane's own %s names the failure and the escalation it filed; stopping the batch", kind, iteration, systemFailureHaltExitCode, CodeLoopSystemFailureHalt),
		map[string]string{"kind": kind, "iteration": strconv.Itoa(iteration), "rc": strconv.Itoa(systemFailureHaltExitCode)})
	b.result.StopReason = stopReason
	b.result.emitFatal(b.stdout, b.stderr, b.cfg, 0)
	return batchDecision{flow: batchReturn, exitCode: exitCode}
}

// failedLaneCount projects the leaf's ONE declaration of the failed-lane
// belief (loopwave.FailedLanes) onto the coordinator's spelling.
func failedLaneCount(results []fleet.Result) int {
	return loopwave.FailedLanes(results)
}
