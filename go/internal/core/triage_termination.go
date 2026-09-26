package core

import (
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

const cycleTerminationTriageClaimFailed = "triage-empty-commitment-claimable-work"

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
	if decision.reason == CycleTerminationTriageNoWork && unansweredClaimableWork(projectRoot, workspace, cycle) {
		decision.reason = cycleTerminationTriageClaimFailed
	}
	if decision.reason != "" && o.floorAlreadyCompleted(completed) {
		decision.reason = ""
	}
	return decision
}

// unansweredClaimableWork reports whether an empty commitment left claimable
// work UNANSWERED — the one shape that is triage's failure rather than the
// queue's honest state. A fleet lane can claim only its scoped items, and a
// scoped item is where triage left it: still pending in the inbox root (triage
// builds its menu there — inboxmover.ClaimLaneScope's placement note) or
// claimed into this cycle's processing/cycle-<N>/ (triage claims before it
// selects). The lane failed only when a scoped item is in one of those places
// AND the decision answered for it nowhere (committedset.Dispositions — an
// escalation, a rejection, a shipped skip, a reasoned drop; never a deferral).
// Judging the root alone would let triage's own claim turn a loud claim
// failure into a silent no-work end (F30 architecture review C1). Cycle 1682's
// lane dropped its already-shipped item with a reason and sealed FAIL because
// the check read the WHOLE inbox: another lane's work is not this lane's claim
// failure (F30). A load warning fails the lane closed only for a scoped item
// the check could not find — a malformed file may be it — so one broken
// unrelated item never fails an answered lane (review m5). A sequential
// cycle's menu is the whole inbox, so it keeps the whole-inbox check.
func unansweredClaimableWork(projectRoot, workspace string, cycle int) bool {
	scope := LaneScopeIDs(workspace)
	if len(scope) == 0 {
		return hasClaimableInboxWork(projectRoot, cycle)
	}
	inbox := filepath.Join(projectRoot, ".evolve", "inbox")
	pending, rootWarned, err := dispatchableIDs(inbox)
	if err != nil {
		return true
	}
	claimed, claimWarned, err := dispatchableIDs(inboxbatch.ProcessingCycleDir(inbox, cycle))
	if err != nil {
		return true
	}
	answered := map[string]bool{}
	for _, d := range committedset.Dispositions(workspace) {
		answered[d.ID] = true
	}
	for _, id := range scope {
		if answered[id] {
			continue
		}
		if pending[id] || claimed[id] || rootWarned || claimWarned {
			return true
		}
	}
	return false
}

// hasClaimableInboxWork is the sequential cycle's check: its menu is the whole
// inbox, so any dispatchable item still pending in the root — or claimed into
// this cycle's processing/ dir and then left uncommitted (the sequential shape
// of review C1) — is claimable work triage failed to commit.
func hasClaimableInboxWork(projectRoot string, cycle int) bool {
	inbox := filepath.Join(projectRoot, ".evolve", "inbox")
	pending, rootWarned, err := dispatchableIDs(inbox)
	if err != nil || rootWarned || len(pending) != 0 {
		return true
	}
	claimed, claimWarned, err := dispatchableIDs(inboxbatch.ProcessingCycleDir(inbox, cycle))
	return err != nil || claimWarned || len(claimed) != 0
}

// dispatchableIDs indexes the lane-dispatchable items in one inbox directory
// (the root, or a cycle's processing/ claim dir); warned reports that some
// file could not be read, which callers treat as "could be anything" — an
// unreadable queue is never proof that nothing was claimable. A missing
// directory is an empty one.
func dispatchableIDs(dir string) (ids map[string]bool, warned bool, err error) {
	items, warnings, err := inboxbatch.LoadDir(dir)
	if err != nil {
		return nil, false, err
	}
	dispatchable, _, _ := inboxbatch.PartitionConsole(items, nil)
	ids = make(map[string]bool, len(dispatchable))
	for _, it := range dispatchable {
		ids[it.ID] = true
	}
	return ids, len(warnings) != 0, nil
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
