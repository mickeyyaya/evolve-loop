package core

import (
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

const cycleTerminationTriageClaimFailed = cyclestate.CycleTerminationTriageClaimFailed

// triageTermination is the host's terminal decision after Triage. Phase
// artifacts supply evidence through router.Digest; the host combines that
// evidence with the canonical phase verdict and prior floor completion.
type triageTermination struct {
	stop   bool
	reason string
}

func decideTriageTermination(verdict string, signals router.RoutingSignals) triageTermination {
	if signals.HasEmptyTriageCommitment() {
		return triageTermination{stop: true, reason: CycleTerminationTriageNoWork}
	}
	if verdict == VerdictFAIL {
		return triageTermination{stop: true}
	}
	return triageTermination{}
}

func (o *Orchestrator) triageTermination(projectRoot, workspace string, cycle int, completed []string, verdict string) triageTermination {
	signals, err := router.Digest(workspace, completed)
	if err != nil {
		return decideTriageTermination(verdict, router.RoutingSignals{})
	}
	decision := decideTriageTermination(verdict, signals)
	if decision.reason == CycleTerminationTriageNoWork && unansweredClaimableWork(projectRoot, workspace, cycle, o.laneMenuOrRoutingOnly()) {
		decision.reason = cycleTerminationTriageClaimFailed
	}
	if decision.reason != "" && o.floorAlreadyCompleted(completed) {
		decision.reason = ""
	}
	return decision
}

func unansweredClaimableWork(projectRoot, workspace string, cycle int, menu LaneMenuFn) bool {
	if len(committedset.LanePin(workspace)) == 0 {
		return hasClaimableInboxWork(projectRoot, cycle, menu)
	}
	inbox := filepath.Join(projectRoot, ".evolve", "inbox")
	pending, rootUnreadable, err := admittedIDs(inbox, inbox, menu)
	if err != nil {
		return true
	}
	claimed, claimUnreadable, err := admittedIDs(inboxbatch.ProcessingCycleDir(inbox, cycle), inbox, routingOnlyLaneMenu)
	if err != nil {
		return true
	}
	for _, id := range committedset.Unanswered(workspace) {
		if pending[id] || claimed[id] || rootUnreadable || claimUnreadable {
			return true
		}
	}
	return false
}

func hasClaimableInboxWork(projectRoot string, cycle int, menu LaneMenuFn) bool {
	inbox := filepath.Join(projectRoot, ".evolve", "inbox")
	pending, rootUnreadable, err := admittedIDs(inbox, inbox, menu)
	if err != nil || rootUnreadable || len(pending) != 0 {
		return true
	}
	claimed, claimUnreadable, err := admittedIDs(inboxbatch.ProcessingCycleDir(inbox, cycle), inbox, routingOnlyLaneMenu)
	return err != nil || claimUnreadable || len(claimed) != 0
}

func admittedIDs(dir, inboxRoot string, admit LaneMenuFn) (ids map[string]bool, unreadable bool, err error) {
	scan, err := inboxbatch.ScanDir(dir)
	if err != nil {
		return nil, false, err
	}
	admitted := admit(inboxRoot, scan.Items)
	ids = make(map[string]bool, len(admitted))
	for _, it := range admitted {
		ids[it.ID] = true
	}
	return ids, scan.HasUnreadable(), nil
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
