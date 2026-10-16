package core

import "time"

var ResumeBoundaryCheckpointer func(cs CycleState, projectRoot string, now time.Time) error

func (cr *cycleRun) applyDispatchPolicy(next Phase, req *PhaseRequest) {
	if next == PhaseBuild {
		if tier, raised := cr.escalatedBuildTier(req.ModelRoutingTier); raised {
			req.ModelRoutingTier = tier
		}
	}
	if tier, raised := cr.o.repairRoundTier(cr.req.ProjectRoot, next, cr.cs, req.ModelRoutingTier); raised {
		req.ModelRoutingTier = tier
	}
	if next == PhaseBuild {
		if scale := cr.buildBudgetScale(); scale != 1.0 {
			req.BudgetScale = scale
		}
	}
}
