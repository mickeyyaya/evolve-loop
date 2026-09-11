package main

import (
	"fmt"
	"os"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
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
		fmt.Fprintf(b.stderr, "[loop] received interrupt (SIGINT/SIGTERM) before cycle %d — stopping; resume with: evolve loop --resume\n", iteration+1)
		b.result.StopReason = "signal"
		b.result.emit(b.stdout)
		return batchDecision{flow: batchReturn, exitCode: 130}
	}
	if exitCode, halted := blockerBreakerHalt(b.cfg.EvolveDir, b.cfg.ProjectRoot, batchStartCycle, b.stderr); halted {
		b.result.StopReason = "pipeline_blocker_halt"
		b.result.emitFatal(b.stdout, b.stderr, b.cfg, 0)
		return batchDecision{flow: batchReturn, exitCode: exitCode}
	}

	runCLIHealthCanary(b.cfg.ProjectRoot, b.cycleEnv, defaultLiveProbe(b.cfg.ProjectRoot, b.stderr), b.stderr)
	runUsageProbe(b.cfg.ProjectRoot, b.cfg.EvolveDir, b.cycleEnv, b.stderr)
	if _, halt := syncMainFromOriginAtWaveBoundary(b.ctx, b.cfg.ProjectRoot, b.stderr); halt != nil {
		fmt.Fprintf(b.stderr, "[loop] HALT: %v\n", halt)
		b.result.StopReason = "plane_diverged_halt"
		b.result.emitFatal(b.stdout, b.stderr, b.cfg, 0)
		return batchDecision{flow: batchReturn, exitCode: 2}
	}

	*fleetConfig = reloadFleetConfigAtWaveBoundary(b.cfg.EvolveDir, *fleetConfig, b.stderr)
	if maybeRefreshChainBoundary(b.cfg, iteration+1, b.stderr) {
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
			applyEscalationBoundary(b.cfg.EvolveDir, iteration, b.stderr)
			return batchDecision{flow: batchNextIteration}
		default:
			fmt.Fprintf(b.stderr, "[loop] WARN: fleet: pool %d planned zero lanes (empty backlog), falling back to sequential\n", iteration)
		}
	}

	if !shouldRunWave(fleetConfig) {
		return batchDecision{flow: batchProceed}
	}
	waveConfig, pace := budgetAwareWaveConfig(b.ctx, fleetConfig, b.cfg.ProjectRoot, b.cfg.EvolveDir, b.deps.Storage, b.stderr)
	launcher := productionWaveLauncher(waveConfig, waveBinary, b.cfg.ProjectRoot, b.cfg.GoalHash, b.cfg.GoalText, b.stdout, b.stderr)
	ran, _, results, err := dispatchIteration(
		b.ctx,
		waveConfig,
		productionWavePreflight(b.cfg.ProjectRoot),
		productionWavePlanFn(b.cfg, b.deps.Storage, waveConfig.Count, b.stderr),
		launcher,
		consoleRoutedResolver(b.cfg.ProjectRoot, b.stderr),
		iteration,
	)
	switch {
	case err != nil:
		fmt.Fprintf(b.stderr, "[loop] WARN: fleet: wave %d dispatch failed, falling back to sequential: %v\n", iteration, err)
	case ran:
		fmt.Fprintf(b.stderr, "[loop] wave %d: %d/%d lanes ok\n", iteration, len(results)-failedLaneCount(results), len(results))
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
		applyEscalationBoundary(b.cfg.EvolveDir, iteration, b.stderr)
		paceBeforeNextWave(b.ctx, pace, b.stderr)
		return batchDecision{flow: batchNextIteration}
	default:
		oneLauncher := productionWaveLauncher(fleetConfig, waveBinary, b.cfg.ProjectRoot, b.cfg.GoalHash, b.cfg.GoalText, b.stdout, b.stderr)
		if minWidthRepair(
			b.ctx,
			fleetConfig,
			waveConfig,
			productionWavePreflight(b.cfg.ProjectRoot),
			productionWavePlanFn(b.cfg, b.deps.Storage, fleetConfig.Count, b.stderr),
			oneLauncher,
			consoleRoutedResolver(b.cfg.ProjectRoot, b.stderr),
			iteration,
			b.stderr,
		) {
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
	fmt.Fprintf(b.stderr, "[loop] SYSTEM-FAILURE HALT: a fleet lane in %s %d exited with the ADR-0072 halt code (rc=%d) — the lane already filed .evolve/pipeline-escalation.json + a P0 pipeline-repair inbox item. Stopping the batch; diagnose the pipeline (not the task) before resuming with evolve loop --resume.\n", kind, iteration, systemFailureHaltExitCode)
	b.result.StopReason = stopReason
	b.result.emitFatal(b.stdout, b.stderr, b.cfg, 0)
	return batchDecision{flow: batchReturn, exitCode: exitCode}
}

func failedLaneCount(results []fleet.Result) int {
	failed := 0
	for _, result := range results {
		if result.Err != nil || result.ExitCode != 0 {
			failed++
		}
	}
	return failed
}
