package router

// floor.go implements the ADR-0024 §1 conditional integrity floor: the SINGLE
// causal invariant that replaces the fixed mandatory-spine never-skip list when
// an advisor drives phase selection (Stage>=Advisory). It is a PURE plan-level
// prefilter — the caller (orchestrator) applies it only when routing is at
// Advisory or above; below that the legacy static path runs unchanged.

// ClampPlanToFloor enforces the conditional integrity floor on an advisory
// whole-cycle plan. The floor is one causal implication:
//
//	reach(ship) ⇒ build ∧ audit ∧ (tdd, unless the cycle is trivial)
//
// If the plan runs ship, the clamp forces build + audit on (and tdd unless the
// cycle is trivial per the configured TDD-pin), recording one Clamp per forced
// phase. If the plan does NOT run ship, nothing is forced — a no-ship cycle may
// legitimately end after scout (investigation/convergence). The clamp can only
// COMPLETE the set, never weaken it (sequencing across the set is the walk's job,
// not the floor's). Phase names must be canonical lowercase — the caller
// normalizes the advisor's parsed output before clamping.
//
// This is a plan-level PREFILTER, not the whole safety story: it forces audit to
// RUN, but the "audit must PASS bound to the built tree" guarantee remains with
// the ship phase's audit-binding (tree-SHA match + EGPS red_count==0) and the
// artifact-backed SpineSatisfiedUpTo gate. Defense in depth — never the sole gate.
//
// PURE: returns a NEW plan (input unmutated) plus the clamps applied.
func ClampPlanToFloor(in RouteInput, plan *PhasePlan) (*PhasePlan, []Clamp) {
	if plan == nil {
		return nil, nil
	}
	// MintPhases carried through unchanged: the clamp governs the run/skip
	// Entries (the integrity floor), never the set of minted phases.
	out := &PhasePlan{
		Entries:    append([]PhasePlanEntry(nil), plan.Entries...),
		MintPhases: plan.MintPhases,
	}

	// No-ship cycle: the implication's antecedent is false, so the floor imposes
	// nothing. scout-only / investigation cycles are legitimate.
	if !planRuns(out, "ship") {
		return out, nil
	}

	var clamps []Clamp
	force := func(phase string) {
		if planRuns(out, phase) {
			return // already running — nothing to clamp
		}
		ensureRun(out, phase)
		clamps = append(clamps, Clamp{
			Rule:     "ship-requires-" + phase,
			Proposed: phase + "=skip",
			Forced:   phase + "=run",
		})
	}

	// Chain order (execution order) for deterministic clamp listing. build + audit
	// are unconditional once ship is reached; tdd carries the trivial exemption.
	if tddPinned(in) {
		force("tdd")
	}
	force("build")
	force("audit")
	return out, clamps
}

// planRuns reports whether the plan has an entry for phase with Run==true. An
// absent phase counts as not-running (the advisor declined to schedule it).
func planRuns(plan *PhasePlan, phase string) bool {
	for _, e := range plan.Entries {
		if e.Phase == phase {
			return e.Run
		}
	}
	return false
}

// ensureRun sets phase's entry to Run==true, appending a forced entry when the
// phase is absent from the plan entirely.
func ensureRun(plan *PhasePlan, phase string) {
	for i := range plan.Entries {
		if plan.Entries[i].Phase == phase {
			plan.Entries[i].Run = true
			return
		}
	}
	plan.Entries = append(plan.Entries, PhasePlanEntry{
		Phase:         phase,
		Run:           true,
		Justification: "floor: ship requires " + phase,
	})
}

// tddPinned reports whether tdd is mandatory this cycle. It reuses the kernel's
// existing conditional-mandatory rule (EVOLVE_CONDITIONAL_MANDATORY, default
// `tdd:cycle_size!=trivial`) so the floor's trivial exemption stays consistent
// with shouldRun's TDD-pin. Absent rule ⇒ pinned (the safer, more-mandatory side).
func tddPinned(in RouteInput) bool {
	if rule, ok := in.Cfg.Conditional["tdd"]; ok {
		return evalCondRule(in.Signals, rule)
	}
	return true
}
