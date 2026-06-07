package router

import "slices"

// EvaluatorFloorPhase is the single non-removable floor phase: a plan can never
// reach ship without an evaluator. It mirrors policy's own constant of the same
// value — each layer independently guarantees the evaluator (defense in depth;
// unifying them would create an import cycle, router/policy.go imports policy),
// so ClampPlanToFloorWith re-asserts it rather than trusting its caller.
// Divergence trips TestEvaluatorFloorPhase_SingleSource.
const EvaluatorFloorPhase = "audit"

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
//
// This is the back-compat entry point: it enforces the SAFE STRUCTURAL DEFAULT
// floor (DefaultShipFloor). Callers that honor a user-configured floor
// (.evolve/policy.json:ship_floor) call ClampPlanToFloorWith with the resolved
// set instead. Keeping this wrapper byte-identical to the historical behavior is
// what lets the existing floor_test.go suite stand as the default-preserving proof.
func ClampPlanToFloor(in RouteInput, plan *PhasePlan) (*PhasePlan, []Clamp) {
	return ClampPlanToFloorWith(in, plan, DefaultShipFloor(), in.IntentRequired)
}

// DefaultShipFloor is the safe structural default: a plan reaching ship must run
// tdd (unless the cycle is trivial), build, and audit. The router owns this
// definition (single source of truth); policy.FloorPhases overrides it only when
// the user supplies an explicit ship_floor.
func DefaultShipFloor() []string { return []string{"tdd", "build", "audit"} }

// ClampPlanToFloorWith enforces a CONFIGURABLE integrity floor on an advisory
// whole-cycle plan: floor is the set of phases a plan reaching ship MUST run.
// For each floor phase the clamp forces it on (recording one Clamp), EXCEPT
// "tdd", which carries the trivial-cycle exemption (forced only when tddPinned).
// Floor order is preserved for deterministic clamp listing. A no-ship plan is
// unconstrained (the implication's antecedent is false). The evaluator phase is
// re-asserted into the floor if absent (self-sealing — see EvaluatorFloorPhase),
// so this function cannot be made to produce a floor without an evaluator even
// by a caller that bypasses policy.FloorPhases. Same defense-in-depth caveat as
// ClampPlanToFloor: this forces audit to RUN; the ship phase's audit-binding
// still guarantees it PASSED.
//
// PURE: returns a NEW plan (input unmutated) plus the clamps applied.
func ClampPlanToFloorWith(in RouteInput, plan *PhasePlan, floor []string, intentRequired bool) (*PhasePlan, []Clamp) {
	if plan == nil {
		return nil, nil
	}
	// Self-sealing evaluator guarantee: re-assert the non-removable evaluator
	// rather than trust the caller, so a future direct caller cannot produce a
	// floor without it. policy.FloorPhases already guarantees this; we do not
	// rely on that.
	if !slices.Contains(floor, EvaluatorFloorPhase) {
		floor = append([]string(nil), floor...)
		floor = append(floor, EvaluatorFloorPhase)
	}
	// MintPhases carried through unchanged: the clamp governs the run/skip
	// Entries (the integrity floor), never the set of minted phases.
	out := &PhasePlan{
		Entries:    append([]PhasePlanEntry(nil), plan.Entries...),
		MintPhases: plan.MintPhases,
	}

	var clamps []Clamp
	force := func(phase string, rule string) {
		if planRuns(out, phase) {
			return // already running — nothing to clamp
		}
		ensureRun(out, phase)
		clamps = append(clamps, Clamp{
			Rule:     rule,
			Proposed: phase + "=skip",
			Forced:   phase + "=run",
		})
	}

	if intentRequired {
		force("intent", "require-intent")
	}

	// No-ship cycle: the implication's antecedent is false, so the floor imposes
	// nothing. scout-only / investigation cycles are legitimate.
	if !planRuns(out, "ship") {
		return out, clamps
	}

	for _, phase := range floor {
		if phase == "tdd" {
			// tdd carries the trivial exemption — forced only when pinned.
			if tddPinned(in) {
				force("tdd", "ship-requires-tdd")
			}
			continue
		}
		force(phase, "ship-requires-"+phase)
	}
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
