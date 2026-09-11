package core

import "github.com/mickeyyaya/evolve-loop/go/internal/router"

// triageTermination is the host's terminal decision after Triage. Phase
// artifacts supply evidence through router.Digest; the host combines that
// evidence with the canonical phase verdict and prior floor completion.
type triageTermination struct {
	stop   bool
	reason string
}

func decideTriageTermination(verdict string, signals router.RoutingSignals) triageTermination {
	if verdict == VerdictFAIL {
		return triageTermination{stop: true}
	}
	if (verdict == VerdictPASS || verdict == VerdictWARN) && signals.HasEmptyTriageCommitment() {
		return triageTermination{stop: true, reason: CycleTerminationTriageNoWork}
	}
	return triageTermination{}
}

func (o *Orchestrator) triageTermination(workspace string, completed []string, verdict string) triageTermination {
	signals, err := router.Digest(workspace, completed)
	if err != nil {
		return decideTriageTermination(verdict, router.RoutingSignals{})
	}
	decision := decideTriageTermination(verdict, signals)
	if decision.reason != "" && o.floorAlreadyCompleted(completed) {
		decision.reason = ""
	}
	return decision
}

func phasesEndAtTriageWithoutImplementation(phases []Phase) bool {
	if len(phases) == 0 || phases[len(phases)-1] != PhaseTriage {
		return false
	}
	for _, phase := range phases {
		switch phase {
		case PhaseTDD, PhaseBuildPlanner, PhaseSwarmPlan, PhaseBuild, PhaseAudit, PhaseShip:
			return false
		}
	}
	return true
}

// IsTriageNoWorkResult reports whether a completed result carries the host's
// planned no-work disposition and proves that no implementation phase ran.
func IsTriageNoWorkResult(result CycleResult) bool {
	return result.FinalVerdict == CycleOutcomeSkippedUnknown &&
		result.TerminationReason == CycleTerminationTriageNoWork &&
		phasesEndAtTriageWithoutImplementation(result.PhasesRun)
}

func shouldWarnSkippedUnknown(result CycleResult) bool {
	return result.FinalVerdict == CycleOutcomeSkippedUnknown &&
		result.TerminationReason != CycleTerminationTriageNoWork
}
