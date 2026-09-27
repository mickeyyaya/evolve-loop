package core

import "context"

// FleetRebaseRecoveryForTest exposes the debugger re-entry guard to the external test package.
func FleetRebaseRecoveryForTest(code string) bool { return fleetRebaseRecovery(code) }

// EarliestPhaseForTest exposes the target choice between the debugger's decision and the rebase route.
func EarliestPhaseForTest(decided, routed Phase) Phase { return earliestPhase(decided, routed) }

// ResumeFleetRebaseAfterDebuggerForTest exposes the re-entry to the external test package.
func (o *Orchestrator) ResumeFleetRebaseAfterDebuggerForTest(ctx context.Context, projectRoot string, cycle int, cs *CycleState, decided Phase, depth, fleetWidth int) (Phase, bool) {
	return o.resumeFleetRebaseAfterDebugger(ctx, projectRoot, cycle, cs, decided, depth, fleetWidth)
}
