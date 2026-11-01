package core

import "time"

var ResumeBoundaryCheckpointer func(cs CycleState, projectRoot string, now time.Time) error

func (cr *cycleRun) applyDispatchPolicy(next Phase, req *PhaseRequest) {
	if next == PhaseBuild {
		if tier, raised := cr.escalatedBuildTier(req.Assignment.Tier); raised {
			req.Assignment.Tier = tier
		}
	}
	if !cr.o.workflowConfig.RetryLadder.Enforced() {
		if tier, raised := cr.o.repairRoundTier(cr.req.ProjectRoot, next, cr.cs, req.Assignment.Tier); raised {
			req.Assignment.Tier = tier
		}
	}
	*req = cr.ladderForDispatch(next, *req)
	if next == PhaseBuild {
		if scale := cr.buildBudgetScale(); scale != 1.0 {
			req.BudgetScale = scale
		}
	}
}
