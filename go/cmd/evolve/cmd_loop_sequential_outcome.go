package main

import (
	"fmt"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclebudget"
)

func (b *loopBatchCoordinator) completeSequentialCycle(iteration int, cycle sequentialCycle, state *sequentialBatchState) batchDecision {
	failed := cycle.result.FinalVerdict == core.VerdictFAIL
	var stop bool
	state.consecutiveFails, stop = consecutiveFailBreaker(failed, state.consecutiveFails, state.maxConsecutiveFails)
	b.closeoutCycleOutcome(cycle.result)
	if stop {
		b.result.StopReason = "fail"
		return batchDecision{flow: batchStopIterations}
	}
	if failed {
		recordAbsorbedFail(b.cfg, cycle.cycle, b.stderr)
		b.result.ContinuedFailures++
		fmt.Fprintf(b.stderr, "[loop] cycle %d verdict=FAIL — continuing (consecutive %d of max %d, workflow policy)\n",
			cycle.cycle, state.consecutiveFails, state.maxConsecutiveFails)
	}

	emptyOrBlocked := cycle.result.FinalVerdict == core.CycleOutcomeSkippedUnknown ||
		cycle.result.FinalVerdict == core.CycleOutcomeSkippedAuditAdvisory
	if escalation := state.goalStall.observe(emptyOrBlocked, cycle.result.FinalVerdict, state.stallCfg.threshold); escalation != nil {
		handleGoalStall(goalStallKind, b.cfg.EvolveDir, b.cfg.GoalHash, cycle.workspace, cycle.cycle, escalation, state.stallCfg.threshold, state.stallCfg.weight, b.stderr)
	}
	if escalation := state.nonprogress.observe(nonShippingOutcome(cycle.result.FinalVerdict), cycle.result.FinalVerdict, state.stallCfg.nonprogressThreshold); escalation != nil {
		handleGoalStall(nonprogressKind, b.cfg.EvolveDir, b.cfg.GoalHash, cycle.workspace, cycle.cycle, escalation, state.stallCfg.nonprogressThreshold, state.stallCfg.weight, b.stderr)
	}
	applyEscalationBoundary(b.cfg.EvolveDir, cycle.cycle, b.stderr, b.deps.Signals)

	if state.budgetStage == cyclebudget.Off || b.cfg.MaxCyclesExplicit || failed {
		return batchDecision{flow: batchNextIteration}
	}
	backlog, ok := readCarryoverCount(filepath.Join(b.cfg.EvolveDir, "state.json"))
	if !ok {
		return batchDecision{flow: batchNextIteration}
	}
	decision := cyclebudget.Next(state.budgetStage, iteration+1, state.effectiveMax, backlog, false)
	if decision.Advisory {
		fmt.Fprintf(b.stderr, "[loop] cycle-budget ADVISORY: would stop (%s) after cycle %d (backlog=%d)\n", decision.Reason, cycle.cycle, backlog)
	}
	if !decision.Stop {
		return batchDecision{flow: batchNextIteration}
	}
	fmt.Fprintf(b.stderr, "[loop] cycle-budget: stopping (%s) after cycle %d (backlog=%d)\n", decision.Reason, cycle.cycle, backlog)
	b.result.StopReason = decision.Reason
	return batchDecision{flow: batchStopIterations}
}

func (b *loopBatchCoordinator) closeoutCycleOutcome(result core.CycleResult) {
	if applied, err := closeoutCycleOutcome(result, b.cfg.ProjectRoot, b.cfg.EvolveDir, b.stderr, b.deps.Ledger, b.deps.Signals); err != nil {
		fmt.Fprintf(b.stderr, "[loop] WARN: could not apply cycle %d %s to the inbox: %v\n", result.Cycle, applied, err)
	}
}

// applyCycleFailureOutcome is the loop's voice for the shared failed-cycle
// inbox walk; every root makes this one call. A lifecycle hiccup WARNs but
// never changes the batch's flow, and the walk's lifecycle lines route
// through the root's ledger so the Signal Center observes them like any
// other entry.
// See ADR-0101, ADR-0103.
func (b *loopBatchCoordinator) applyCycleFailureOutcome(cycle int) {
	if err := applyCycleFailureOutcome(b.cfg.ProjectRoot, b.cfg.EvolveDir, cycle, b.stderr, b.deps.Ledger, b.deps.Signals); err != nil {
		fmt.Fprintf(b.stderr, "[loop] WARN: could not apply cycle %d failure outcome to the inbox: %v\n", cycle, err)
	}
}
