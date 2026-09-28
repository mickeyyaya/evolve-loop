package core

import "time"

// ResumeBoundaryCheckpointer writes an operator-requested active-phase
// checkpoint under the sidecar lock. It advances an already-consumed pause on
// resume and preserves the in-flight phase on a graceful fresh-cycle interrupt.
// Unlike the routine boundary writer, it must replace the previous escalation.
// The checkpoint package registers the durable writer.
var ResumeBoundaryCheckpointer func(cs CycleState, projectRoot string, now time.Time) error

// applyDispatchPolicy is shared by fresh and resumed phase dispatch. The
// persisted task scope and completed triage artifacts drive the same floors.
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
