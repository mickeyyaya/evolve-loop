package router

// PlanRejection reports one structural problem ValidatePlan found in an advisory
// whole-cycle plan. It is pure telemetry — ValidatePlan NEVER mutates the plan,
// so the integrity floor (ClampPlanToFloorWith) remains the sole disposer. Reason
// is a stable token for metrics; Detail is the human one-liner. Phase is empty
// for whole-plan problems (e.g. an empty plan).
type PlanRejection struct {
	Phase  string
	Reason string // empty-plan | unknown-phase | duplicate-phase | ship-skips-audit
	Detail string
}

// ValidatePlan reports structural problems in an advisory whole-cycle plan
// (ADR-0052 WS2-S1; research principle P6 — validate before clamp). It is PURE
// and REPORT-ONLY: it never mutates the plan, never widens the run-set, and sits
// strictly ABOVE the integrity floor — ClampPlanToFloorWith still runs last,
// unconditionally, as the sole trust boundary. The orchestrator uses the
// rejections for telemetry (WS2-S2) and, once the re-plan is at advisory, to
// refuse a malformed re-plan back to the prior clamped plan rather than acting
// on garbage.
//
// Mint-aware (must-fix): a phase minted IN THIS PLAN counts as known, never
// "unknown". Checks, in order: empty plan; per entry — duplicate name, unknown
// name; and finally a run:true ship while audit is not scheduled (the floor will
// force audit, but surfacing the advisor's intent keeps the decision debuggable).
func ValidatePlan(in RouteInput, plan *PhasePlan) []PlanRejection {
	if plan == nil || len(plan.Entries) == 0 {
		return []PlanRejection{{Reason: "empty-plan", Detail: "plan has no entries"}}
	}
	known := knownPhaseSet(in, plan)

	var rej []PlanRejection
	seen := make(map[string]struct{}, len(plan.Entries))
	shipRuns, auditRuns := false, false
	for _, e := range plan.Entries {
		if _, dup := seen[e.Phase]; dup {
			rej = append(rej, PlanRejection{Phase: e.Phase, Reason: "duplicate-phase", Detail: "phase appears more than once in the plan"})
			continue
		}
		seen[e.Phase] = struct{}{}
		if _, ok := known[e.Phase]; !ok {
			rej = append(rej, PlanRejection{Phase: e.Phase, Reason: "unknown-phase", Detail: "not a canonical, configured, or minted phase"})
		}
		switch {
		case e.Phase == "ship" && e.Run:
			shipRuns = true
		case e.Phase == EvaluatorFloorPhase && e.Run:
			auditRuns = true
		}
	}
	if shipRuns && !auditRuns {
		rej = append(rej, PlanRejection{Phase: "ship", Reason: "ship-skips-audit", Detail: "ship runs but " + EvaluatorFloorPhase + " is not scheduled (the floor will force it)"})
	}
	return rej
}

// PlanMismatch reports whether the MEASURED signals materially diverge from what
// the plan scheduled (ADR-0052 WS2-S4; research P4 TAPE — mismatch-triggered
// replanning). It is true ONLY when an optional phase whose insert_when trigger
// now FIRES on the measured signals is NOT scheduled in the plan — i.e. the
// initial plan (composed with empty signals) missed a need the post-scout
// measurement reveals. It reuses triggerFires (the exact insert_when eval the
// kernel walks), so the mismatch threshold can never disagree with the trigger.
// A fired trigger the plan ALREADY covers is not a mismatch (re-planning would be
// churn); a nil plan ⇒ no mismatch. PURE.
func PlanMismatch(in RouteInput, plan *PhasePlan) bool {
	if plan == nil {
		return false
	}
	for phase, block := range in.Cfg.Triggers {
		if triggerFires(in.Signals, block) && !planRuns(plan, phase) {
			return true
		}
	}
	return false
}

// knownPhaseSet is the set of phase names a plan may legitimately reference: the
// built-in canonical order, the configured walk order + mandatory + trigger +
// conditional phases (which already include any catalog phases spliced in at the
// composition root), and — mint-aware — the phases minted in THIS plan. Reusing
// in.Cfg avoids a parallel notion of "known phase" drifting from the walk.
func knownPhaseSet(in RouteInput, plan *PhasePlan) map[string]struct{} {
	known := make(map[string]struct{}, len(canonicalOrder)+len(in.Cfg.Order)+len(plan.MintPhases))
	add := func(names ...string) {
		for _, n := range names {
			if n != "" {
				known[n] = struct{}{}
			}
		}
	}
	add(canonicalOrder...)
	add(in.Cfg.Order...)
	add(in.Cfg.Mandatory...)
	for p := range in.Cfg.Triggers {
		add(p)
	}
	for p := range in.Cfg.Conditional {
		add(p)
	}
	for _, m := range plan.MintPhases {
		add(m.Name)
	}
	return known
}
