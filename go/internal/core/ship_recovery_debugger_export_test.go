package core

import "context"

func FleetRebaseRecoveryForTest(code string) bool { return fleetRebaseRecovery(code) }

func EarliestPhaseForTest(decided, routed Phase) Phase { return earliestPhase(decided, routed) }

func (o *Orchestrator) ResumeFleetRebaseAfterDebuggerForTest(ctx context.Context, projectRoot string, cycle int, cs *CycleState, decided Phase, depth, fleetWidth int) (Phase, bool) {
	return o.resumeFleetRebaseAfterDebugger(ctx, projectRoot, cycle, cs, decided, depth, fleetWidth)
}
