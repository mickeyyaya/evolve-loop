package core

import "fmt"

func (o *Orchestrator) resumeAfterAudit(cs *CycleState, cursor *resumeCursor, result *CycleResult, timings []phaseTimingEntry, verdict string) error {
	if cursor.current == PhaseAudit && verdict == VerdictFAIL {
		branch, reason, sysFail := o.decideAfterAuditFail(*cs)
		consumeAuditRepairGrant(cs, reason)
		if (&cycleRun{o: o, cs: *cs, cycle: cs.CycleID}).sealHandedOff(result, timings, reason) {
			cursor.termination.reason = CycleTerminationHandedOff
		}
		if sysFail != nil && result.SystemFailure == nil {
			result.SystemFailure = sysFail
		}
		if branch != PhaseRetro {
			if !o.sm.CanTransition(PhaseAudit, branch) {
				return fmt.Errorf("audit→%s not allowed by state machine", branch)
			}
			cursor.schedule(branch)
		}
	}
	if branch, ok := o.decideExplanationCorrection(cs, cursor.current, verdict); ok {
		cursor.schedule(branch)
	}
	return nil
}
