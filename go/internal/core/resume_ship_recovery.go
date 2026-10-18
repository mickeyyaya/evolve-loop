package core

import (
	"maps"

	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
)

var resumeRecoveryRoutes = map[Phase]bool{PhaseBuild: true, PhaseAudit: true, PhaseShip: true}

func (cr *cycleRun) recoverResumedShip(next Phase, err error, resp PhaseResponse, attempts int) (Phase, bool) {
	if !cr.recoverShipError(next, cr.staleBaseAsRebase(err), resp, attempts) {
		return "", false
	}
	return cr.scheduledNext, resumeRecoveryRoutes[cr.scheduledNext]
}

func (cr *cycleRun) staleBaseAsRebase(err error) error {
	se, ok := AsShipError(err)
	if !ok || !isLandingDivergence(se) || !cr.baseBehindMain() {
		return err
	}
	debug := maps.Clone(se.Debug)
	debug[shiperr.OriginalCodeKey] = string(se.Code)
	return &ShipError{
		Code:    CodeGitFleetRebaseNeeded,
		Class:   ShipClassTransient,
		Stage:   se.Stage,
		Message: "resumed ship found main past the cycle base " + cr.cs.WorktreeBaseSHA + "; rebase and re-verify the merged tree: " + se.Message,
		Debug:   debug,
	}
}

func isLandingDivergence(se *ShipError) bool {
	branch := se.Debug[shiperr.BranchKey]
	return se.Code == CodeGitFFMergeDiverged && se.Debug[shiperr.CycleBranchKey] != "" && (branch == "" || branch == "main")
}

func (cr *cycleRun) baseBehindMain() bool {
	base := cr.cs.WorktreeBaseSHA
	if cr.cs.ActiveWorktree == "" || !objectIDRe.MatchString(base) {
		return false
	}
	return cr.isAncestor(base, "main") && !cr.isAncestor("main", base)
}

func (cr *cycleRun) isAncestor(ancestor, descendant string) bool {
	_, exit, err := gitCapture(cr.ctx, cr.cs.ActiveWorktree, "merge-base", "--is-ancestor", ancestor, descendant)
	return err == nil && exit == 0
}
