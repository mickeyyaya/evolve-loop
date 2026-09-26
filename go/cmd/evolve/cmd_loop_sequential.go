package main

import "github.com/mickeyyaya/evolve-loop/go/internal/cyclebudget"

type sequentialBatchState struct {
	dispPolicy          dispatchPolicy
	threshold           int
	prevRanCycle        int
	sameCycleStreak     int
	maxConsecutiveFails int
	consecutiveFails    int
	budgetStage         cyclebudget.Stage
	effectiveMax        int
	goalStall           goalStallTracker
	nonprogress         goalStallTracker
	stallCfg            goalStallConfig
}

func (b *loopBatchCoordinator) runSequentialIteration(iteration int, state *sequentialBatchState) batchDecision {
	cycle, decision := b.dispatchSequentialCycle()
	if decision.flow != batchProceed {
		return decision
	}
	if decision = b.observeSequentialCycle(cycle, state); decision.flow != batchProceed {
		return decision
	}
	if decision = b.verifySequentialCycle(cycle, state); decision.flow != batchProceed {
		return decision
	}
	return b.completeSequentialCycle(iteration, cycle, state)
}
