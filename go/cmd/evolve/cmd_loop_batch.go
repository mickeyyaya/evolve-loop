package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclebudget"
	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/loopwave"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type loopBatchCoordinator struct {
	ctx              context.Context
	cfg              loopConfig
	deps             orchDeps
	orch             loopCycleRunner
	cycleEnv         map[string]string
	cycleContext     map[string]string
	result           *loopResult
	workflow         policy.WorkflowConfig
	lastBeforeGCHook int
	stdout           io.Writer
	stderr           io.Writer
	// waveEngine is lazily built by wave().
	// See ADR-0103.
	waveEngine *loopwave.Engine
}

func (b *loopBatchCoordinator) run() int {
	cfg := b.cfg
	deps := b.deps
	lr := b.result
	lastBeforeGCHook := b.lastBeforeGCHook
	stdout := b.stdout
	stderr := b.stderr
	wc := b.workflow

	dc := loadDispatchConfig(cfg.EvolveDir)
	dispPolicy := resolveDispatchPolicy(dc.Policy, stderr)
	threshold := resolveCircuitBreakerThreshold(dc.RepeatThreshold)

	// prevRanCycle/sameCycleStreak trip the breaker on consecutive identical
	// cycle numbers past threshold.
	prevRanCycle := -1
	sameCycleStreak := 0

	// maxConsecutiveFails default 1 stops the batch on the first FAIL; a
	// higher value absorbs isolated verdict misses while the streak cap
	// still halts a genuinely broken run.
	maxConsecutiveFails := wc.MaxConsecutiveFails
	consecutiveFails := 0

	// budgetStage off honors the operator's --max-cycles unchanged; enforce
	// with no explicit --max-cycles raises the ceiling to the safety cap and
	// lets backlog completion drive the early stop; advisory only computes
	// and logs.
	budgetStage := cyclebudget.ParseStage(wc.CycleBudget)
	effectiveMax := cfg.MaxCycles
	if budgetStage == cyclebudget.Enforce && !cfg.MaxCyclesExplicit {
		effectiveMax = wc.MaxCyclesCap
		fmt.Fprintf(stderr, "[loop] no --cycles: advisor-decided, completion-driven (stops when the backlog drains), safety cap=%d\n", effectiveMax)
	}

	fleetCfg := loadFleetConfig(cfg.EvolveDir)
	for _, w := range fleetCfg.Warnings {
		fmt.Fprintf(stderr, "[loop] WARN: fleet: %s\n", w)
	}
	var waveBinPath string
	if shouldRunWave(fleetCfg) || shouldRunPool(fleetCfg) {
		if bp, err := os.Executable(); err == nil {
			waveBinPath = bp
		} else {
			fmt.Fprintf(stderr, "[loop] WARN: fleet: cannot resolve binary for fleet dispatch, staying sequential: %v\n", err)
			fleetCfg.Count = 1
		}
	}

	// starvationTracker is held across the whole batch loop so a starved
	// streak spans waves; a sequential (Count==1) batch never touches it.
	var starvationTracker fleet.StarvationTracker

	// goalStall and nonprogress are held outside the loop so a streak spans
	// cycles. Sequential path only: a fleet lane is a separate process and
	// advances neither counter (a standing breaker-parity gap).
	var goalStall, nonprogress goalStallTracker
	stallCfg := loadGoalStallConfig(cfg.EvolveDir)

	batchStartCycle, _ := readBatchWindowFloor(context.Background(), deps.Storage)
	sequentialState := sequentialBatchState{
		dispPolicy:          dispPolicy,
		threshold:           threshold,
		prevRanCycle:        prevRanCycle,
		sameCycleStreak:     sameCycleStreak,
		maxConsecutiveFails: maxConsecutiveFails,
		consecutiveFails:    consecutiveFails,
		budgetStage:         budgetStage,
		effectiveMax:        effectiveMax,
		goalStall:           goalStall,
		nonprogress:         nonprogress,
		stallCfg:            stallCfg,
	}

iterations:
	for i := 0; i < effectiveMax; i++ {
		if decision := b.prepareIteration(i, &fleetCfg, &waveBinPath, batchStartCycle); decision.flow == batchReturn {
			return decision.exitCode
		}
		switch decision := b.dispatchFleetIteration(i, fleetCfg, waveBinPath, &starvationTracker); decision.flow {
		case batchNextIteration:
			continue
		case batchReturn:
			return decision.exitCode
		}
		switch decision := b.runSequentialIteration(i, &sequentialState); decision.flow {
		case batchNextIteration:
			continue
		case batchStopIterations:
			break iterations
		case batchReturn:
			return decision.exitCode
		}
	}

	if lr.StopReason == "error" || lr.StopReason == "fail" {
		lr.emitFatal(stdout, stderr, cfg, lastCycleIn(*lr))
		return 2
	}
	finalizeCompletedCycle(cfg, stderr)
	gcHookFn(cfg, cycleWorkspace(cfg.ProjectRoot, batchEndGCCycle(*lr, lastBeforeGCHook+1)), stderr)
	lr.emit(stdout)
	// rc=3 signals a batch that completed but absorbed a recoverable failure
	// or a continued verdict-FAIL, so CI can distinguish it from a clean run.
	if lr.RecoverableFailures > 0 || lr.ContinuedFailures > 0 {
		return 3
	}
	return 0
}
