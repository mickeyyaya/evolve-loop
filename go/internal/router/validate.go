package router

// PlanRejection is one problem found in a plan, or one clamp, as recorded in advisor-rejections.json.
// Phase is empty for a whole-plan problem.
type PlanRejection struct {
	Phase  string
	Reason string // a stable token, e.g. empty-plan | unknown-phase | duplicate-phase | ship-skips-audit
	Detail string
}

// ValidatePlan reports structural problems in a plan without changing it; ClampPlanToFloorWith
// stays the sole disposer. A phase minted in this plan counts as known.
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

// PlanMismatch reports whether a trigger now fires for a phase the plan does not run, the signal to re-plan.
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

// knownPhaseSet is every phase a plan may reference. It is deliberately generous, because the
// unknown-phase drop must never delete a legitimate phase.
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
	// A phase the plan prompt offered is known by construction, even though Cfg.Order carries it too.
	for _, c := range in.Catalog {
		add(c.Name)
	}
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
	// Phases minted inline on their own entry, the second minting channel.
	for _, e := range plan.Entries {
		if e.Mint != nil {
			add(e.Phase)
		}
	}
	return known
}
