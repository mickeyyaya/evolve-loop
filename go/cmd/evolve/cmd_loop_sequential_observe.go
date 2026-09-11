package main

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclecost"
	"github.com/mickeyyaya/evolve-loop/go/internal/dispatchevents"
)

// observeSequentialCycle applies terminal system, quota, and same-cycle
// breakers before the ledger verification path can mask them.
func (b *loopBatchCoordinator) observeSequentialCycle(cycle sequentialCycle, state *sequentialBatchState) batchDecision {
	if failure := cycle.result.SystemFailure; failure != nil && failure.Halt {
		exitCode := haltOnSystemFailure(b.cfg.EvolveDir, b.cfg.ProjectRoot, cycle.cycle, cycle.workspace, failure, b.stderr)
		b.result.StopReason = "system_failure_halt"
		b.result.emitFatal(b.stdout, b.stderr, b.cfg, cycle.cycle)
		return batchDecision{flow: batchReturn, exitCode: exitCode}
	}
	if cycle.lastAfter <= cycle.lastBefore {
		if dirExists(cycle.workspace) {
			writer := dispatchevents.NewWriter(cycle.workspace)
			_ = writer.EmitCounterNonAdvance(cycle.cycle)
		}
		fmt.Fprintf(b.stderr, "[loop] NOTE: lastCycleNumber did not advance after cycle %d — verdict likely WARN/FAIL\n", cycle.cycle)
	}
	if cost, err := cyclecost.SummarizeCycle(cycle.workspace, cycle.cycle); err == nil {
		b.result.TotalCost += cost.Total.CostUSD
		fmt.Fprintf(b.stderr, "[loop] cycle %d cost: $%.4f (batch total: $%.4f)\n", cycle.cycle, cost.Total.CostUSD, b.result.TotalCost)
	}
	if pause, ok := detectQuotaPause(b.cfg.EvolveDir); ok {
		fmt.Fprintf(b.stderr, "QUOTA-PAUSE: cycle=%d wake-at=%s source=%s attempts=%d/%d\n",
			pause.Cycle, pause.WakeAt, pause.Source, pause.Attempts, pause.MaxAttempts)
		fmt.Fprintln(b.stderr, "[loop]   to auto-resume in-session: SKILL.md / /loop wrapper calls ScheduleWakeup until wake-at then /evo:loop --resume")
		fmt.Fprintln(b.stderr, "[loop]   to resume manually: evolve loop --resume")
		b.result.StopReason = "quota-pause"
		b.result.emit(b.stdout)
		return batchDecision{flow: batchReturn, exitCode: 5}
	}

	var tripped bool
	state.prevRanCycle, state.sameCycleStreak, tripped = updateBreaker(
		state.prevRanCycle, state.sameCycleStreak, cycle.cycle, state.threshold,
	)
	if !tripped {
		return batchDecision{flow: batchProceed}
	}
	fmt.Fprintf(b.stderr, "[loop] ABORT: same cycle number (%d) reported %d consecutive times (threshold=%d) — dispatcher deadlocked\n", cycle.cycle, state.sameCycleStreak, state.threshold)
	if dirExists(cycle.workspace) {
		writer := dispatchevents.NewWriter(cycle.workspace)
		_ = writer.EmitCircuitBreakerTripped(cycle.cycle, state.sameCycleStreak, state.threshold)
	}
	b.result.StopReason = "circuit_breaker"
	b.result.emitFatal(b.stdout, b.stderr, b.cfg, cycle.cycle)
	return batchDecision{flow: batchReturn, exitCode: 1}
}
