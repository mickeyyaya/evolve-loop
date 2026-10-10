package router

type PlanRejection struct {
	Phase  string
	Reason string
	Detail string
}

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
	for _, e := range plan.Entries {
		if e.Mint != nil {
			add(e.Phase)
		}
	}
	return known
}
