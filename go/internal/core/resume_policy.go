package core

import "time"

// ResumeBoundaryCheckpointer advances an already-consumed pause under the
// checkpoint sidecar lock. Unlike the routine boundary writer, this must replace
// the previous escalation. The checkpoint package registers the durable writer.
var ResumeBoundaryCheckpointer func(cs CycleState, projectRoot string, now time.Time) error

// applyDispatchPolicy is shared by fresh and resumed phase dispatch. The
// persisted task scope and completed triage artifacts drive the same floors.
func (cr *cycleRun) applyDispatchPolicy(next Phase, req *PhaseRequest) {
	// ADR-0076 D (deterministic escalation floor — applied AFTER the mode-
	// gated projection and INDEPENDENT of it, review finding D1: the live
	// registry runs model_routing=static and a policy floor must still fire):
	// a retried scoped item raises the build dispatch tier to deep, clamped
	// through the same envelope guardrail as the routing clamp.
	if next == PhaseBuild {
		if tier, raised := cr.escalatedBuildTier(req.ModelRoutingTier); raised {
			req.ModelRoutingTier = tier
		}
	}
	// Audit-repair round (research R1): a tdd/build re-entry while
	// AuditRepairActive raises to the profile's declared audit_retry_2plus tier
	// — same state the repair brief derives from, same envelope clamp as above.
	if tier, raised := cr.o.repairRoundTier(cr.req.ProjectRoot, next, cr.cs, req.ModelRoutingTier); raised {
		req.ModelRoutingTier = tier
	}
	// ADR-0076 slice A: the build dispatch carries the cycle's difficulty
	// multiplier so the engine can stretch the artifact-wait deadline. Scale
	// 1.0 is left unset — byte-identical legacy dispatch.
	if next == PhaseBuild {
		if scale := cr.buildBudgetScale(); scale != 1.0 {
			req.BudgetScale = scale
		}
	}
}
