package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/loopchain"
	"github.com/mickeyyaya/evolve-loop/go/internal/loopwave"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
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

const loopOperatorBrakeStop = "loop_operator_brake"

// prepareIteration refreshes every batch-wide input at the only safe boundary:
// after the previous window drained and before the next dispatch begins.
func (b *loopBatchCoordinator) prepareIteration(iteration int, fleetConfig *policy.FleetConfig, waveBinary *string, batchStartCycle int) batchDecision {
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
	if loopchain.BrakeEngaged(b.cfg.EvolveDir) {
		return b.brakeStop(iteration)
	}
	if decision, stop := b.runPreWave(iteration); stop {
		return decision
	}

	*fleetConfig = loopwave.ReloadFleetConfig(b.cfg.EvolveDir, *fleetConfig, b.stderr)
	if b.maybeRefreshChainBoundaryAtWave(iteration) {
		b.result.StopReason = "loop_boundary_refresh_reexec"
		if entry, err := lastChainBoundaryRefreshLogEntry(b.cfg.EvolveDir); err == nil {
			b.result.BoundaryRefresh = entry
		}
		b.result.emit(b.stdout)
		return batchDecision{flow: batchReturn}
	}
	if b.ctx.Err() != nil {
		return b.interruptReturn(iteration, "during the boundary refresh ")
	}

	b.resolveWaveBinary(fleetConfig, waveBinary)
	return batchDecision{flow: batchProceed}
}

func (b *loopBatchCoordinator) runPreWave(iteration int) (batchDecision, bool) {
	if b.preWave != nil {
		return b.preWave(iteration)
	}
	return b.probeSyncAndPublish(iteration)
}

func (b *loopBatchCoordinator) probeSyncAndPublish(iteration int) (batchDecision, bool) {
	// An interrupt that landed during the probes must not dispatch a wave
	// that would be cancelled at spawn.
	if err := runPreWaveProbes(b.ctx, b.cfg.ProjectRoot, b.cfg.EvolveDir, b.cycleEnv, b.stderr); err != nil {
		return b.interruptReturn(iteration, "during the pre-wave probes "), true
	}
	if _, halt := syncMainFromOriginAtWaveBoundary(b.ctx, b.cfg.ProjectRoot, b.stderr); halt != nil {
		b.result.StopReason = "plane_diverged_halt"
		if errors.Is(halt, errMainCIRed) {
			b.result.StopReason = "main_ci_red_halt"
		}
		emitLoopHalt(b.deps.Signals, 0, "loopBatchCoordinator.probeSyncAndPublish", CodeLoopHalt, halt.Error(), map[string]string{"stop_reason": b.result.StopReason})
		b.result.emitFatal(b.stdout, b.stderr, b.cfg, 0)
		return batchDecision{flow: batchReturn, exitCode: 2}, true
	}
	publishPendingDossiers(b.cfg.ProjectRoot, b.stderr)
	return batchDecision{flow: batchProceed}, false
}

func (b *loopBatchCoordinator) maybeRefreshChainBoundaryAtWave(iteration int) bool {
	return wiredRefresher(b.cfg, b.stderr, b.deps.Signals, loopchain.WithHandoff(loopchain.Handoff{PID: os.Getpid(), WavesDone: iteration})).Refresh(b.ctx, iteration+1)
}

func (b *loopBatchCoordinator) brakeStop(iteration int) batchDecision {
	fmt.Fprintf(b.stderr, "[loop] operator brake %s is engaged — stopping before cycle %d; release it with: evolve loop-stop --release\n", paths.LoopStopPath(b.cfg.EvolveDir), iteration+1)
	b.result.StopReason = loopOperatorBrakeStop
	return batchDecision{flow: batchStopIterations}
}

func (b *loopBatchCoordinator) resolveWaveBinary(fleetConfig *policy.FleetConfig, waveBinary *string) {
	if !(shouldRunWave(*fleetConfig) || shouldRunPool(*fleetConfig)) || *waveBinary != "" {
		return
	}
	binary, err := os.Executable()
	if err != nil {
		fmt.Fprintf(b.stderr, "[loop] WARN: fleet: cannot resolve binary for fleet dispatch, staying sequential: %v\n", err)
		fleetConfig.Count = 1
		return
	}
	*waveBinary = binary
}

// interruptReturn reports a SIGINT/SIGTERM caught at an iteration boundary
// check point and returns the decision that stops the batch cleanly.
func (b *loopBatchCoordinator) interruptReturn(iteration int, when string) batchDecision {
	signalStop(b.stdout, b.stderr, b.result, fmt.Sprintf("%sbefore cycle %d — stopping", when, iteration+1))
	return batchDecision{flow: batchReturn, exitCode: 130}
}

func (b *loopBatchCoordinator) dispatchFleetIteration(
	iteration int,
	fleetConfig policy.FleetConfig,
	waveBinary string,
	starvation *fleet.StarvationTracker,
) batchDecision {
	if shouldRunPool(fleetConfig) {
		if decision, done := b.dispatchPool(iteration, fleetConfig, waveBinary); done {
			return decision
		}
	}
	if !shouldRunWave(fleetConfig) {
		return batchDecision{flow: batchProceed}
	}
	waveConfig, pace := budgetAwareWaveConfig(b.ctx, fleetConfig, b.cfg.ProjectRoot, b.cfg.EvolveDir, b.deps.Storage, b.stderr)
	out, err := b.wave().Dispatch(b.ctx, b.waveRequest(iteration, waveConfig, waveBinary))
	switch {
	case err != nil:
		return batchDecision{flow: batchProceed}
	case out.Ran:
		return b.completeWave(iteration, fleetConfig, waveConfig, out.Results, pace, starvation)
	default:
		return b.repairMinWidth(iteration, fleetConfig, waveConfig, waveBinary, starvation)
	}
}

func (b *loopBatchCoordinator) dispatchPool(iteration int, fleetConfig policy.FleetConfig, waveBinary string) (batchDecision, bool) {
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
			return decision, true
		}
		applyEscalationBoundary(b.cfg.EvolveDir, iteration, b.stderr, b.deps.Signals)
		return batchDecision{flow: batchNextIteration}, true
	default:
		fmt.Fprintf(b.stderr, "[loop] WARN: fleet: pool %d planned zero lanes (empty backlog), falling back to sequential\n", iteration)
	}
	return batchDecision{}, false
}

func (b *loopBatchCoordinator) waveRequest(iteration int, cfg policy.FleetConfig, waveBinary string) loopwave.DispatchRequest {
	return loopwave.DispatchRequest{
		Config:    cfg,
		Wave:      iteration,
		Preflight: productionWavePreflight(b.cfg.ProjectRoot),
		Plan:      b.wave().PlanFn(cfg.Count),
		Launcher:  b.wave().Launcher(iteration, cfg.Concurrency, execCycleLaunch(waveBinary, false, b.cfg.ProjectRoot, b.cfg.GoalHash, b.cfg.GoalText, b.stdout, b.stderr)),
		Routed:    b.wave().RoutedResolver(),
	}
}

func (b *loopBatchCoordinator) completeWave(iteration int, fleetConfig, waveConfig policy.FleetConfig, results []fleet.Result, pace time.Duration, starvation *fleet.StarvationTracker) batchDecision {
	lanesOK := len(results) - failedLaneCount(results)
	fmt.Fprintf(b.stderr, "[loop] wave %d: %d/%d lanes ok\n", iteration, lanesOK, len(results))
	emitLoopWave(b.deps.Signals, iteration, "loopBatchCoordinator.completeWave", "",
		fmt.Sprintf("wave %d: %d/%d lanes ok", iteration, lanesOK, len(results)),
		map[string]string{"lanes_ok": strconv.Itoa(lanesOK), "lanes": strconv.Itoa(len(results))})
	if decision := b.fleetHaltDecision("wave", iteration, results); decision.flow == batchReturn {
		return decision
	}
	b.observeWorkSupply(iteration, fleetConfig, waveConfig, len(results), starvation)
	applyEscalationBoundary(b.cfg.EvolveDir, iteration, b.stderr, b.deps.Signals)
	paceBeforeNextWave(b.ctx, pace, b.stderr)
	return batchDecision{flow: batchNextIteration}
}

func (b *loopBatchCoordinator) repairMinWidth(iteration int, fleetConfig, waveConfig policy.FleetConfig, waveBinary string, starvation *fleet.StarvationTracker) batchDecision {
	repaired := b.wave().RepairMinWidth(b.ctx, fleetConfig, waveConfig, b.waveRequest(iteration, fleetConfig, waveBinary))
	b.observeWorkSupply(iteration, fleetConfig, waveConfig, repairedLanes(repaired), starvation)
	if repaired {
		return batchDecision{flow: batchNextIteration}
	}
	return batchDecision{flow: batchProceed}
}

func (b *loopBatchCoordinator) observeWorkSupply(iteration int, fleetConfig, waveConfig policy.FleetConfig, realized int, starvation *fleet.StarvationTracker) {
	observation := fleet.WaveObservation{
		DesiredLanes:  fleetConfig.Count,
		RealizedLanes: realized,
		QuotaShrunk:   waveConfig.Count < fleetConfig.Count,
	}
	if !starvation.Observe(observation, fleetConfig.StarvationK) {
		return
	}
	item := fleet.BuildStarvationItem(observation, fleetConfig.StarvationK, fleetConfig.StarvationWeight, iteration, time.Now().UTC().Format(time.RFC3339))
	path, err := item.WriteTo(b.cfg.EvolveDir)
	if err != nil {
		fmt.Fprintf(b.stderr, "[loop] WARN: fleet: could not self-file starvation todo: %v\n", err)
		return
	}
	fmt.Fprintf(b.stderr, "[loop] fleet: work-supply starvation after %d waves — self-filed %s\n", fleetConfig.StarvationK, path)
}

func repairedLanes(repaired bool) int {
	if repaired {
		return loopwave.RepairWidth
	}
	return 0
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
