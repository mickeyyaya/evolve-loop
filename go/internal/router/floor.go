package router

import "slices"

// EvaluatorFloorPhase is the floor phase no configuration can remove; policy holds a pinned twin.
// See ADR-0060.
const EvaluatorFloorPhase = "audit"

// ClampPlanToFloor enforces the default integrity floor (DefaultShipFloor) on an advisory plan.
func ClampPlanToFloor(in RouteInput, plan *PhasePlan) (*PhasePlan, []Clamp) {
	return ClampPlanToFloorWith(in, plan, DefaultShipFloor(), in.IntentRequired)
}

// DefaultShipFloor returns the phases a plan reaching ship must run unless policy sets ship_floor.
func DefaultShipFloor() []string { return []string{"tdd", "build", "audit"} }

// ClampPlanToFloorWith drops unknown and unavailable entries, then forces intent when required, audit
// and ship onto a building plan, and floor onto a ship-bound one. It returns a new plan and one Clamp
// per change; phase names must be canonical.
// See ADR-0024.
func ClampPlanToFloorWith(in RouteInput, plan *PhasePlan, floor []string, intentRequired bool) (*PhasePlan, []Clamp) {
	if plan == nil {
		return nil, nil
	}
	// Re-assert the evaluator rather than trust policy.FloorPhases to have added it.
	if !slices.Contains(floor, EvaluatorFloorPhase) {
		floor = append([]string(nil), floor...)
		floor = append(floor, EvaluatorFloorPhase)
	}
	entries, clamps := dropUnknownPhases(in, plan)
	out := &PhasePlan{
		Entries:    entries,
		MintPhases: plan.MintPhases,
	}
	// A validated unified commitment makes these two design phases deep work.
	// Empty means absent or rejected, preserving triage's fail-open behavior.
	if in.Signals.Triage.UnifiedSize != "" {
		for i := range out.Entries {
			e := &out.Entries[i]
			if (e.Phase != "plan-review" && e.Phase != "build-planner") || e.Tier == "deep" {
				continue
			}
			proposed := e.Phase + "=tier:" + e.Tier
			e.Tier = "deep"
			clamps = append(clamps, Clamp{
				Phase:    e.Phase,
				Rule:     "unified-commitment-planning-tier",
				Proposed: proposed,
				Forced:   e.Phase + "=tier:deep",
			})
		}
	}

	force := func(phase string, rule string) {
		if planRuns(out, phase) {
			return
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

	// Built work may not strand unreviewed or unshipped. Forcing ship makes the
	// plan ship-bound, so the ship floor below completes the set.
	if planRuns(out, "build") {
		force(EvaluatorFloorPhase, "build-requires-"+EvaluatorFloorPhase)
		force("ship", "build-requires-ship")
	}

	// A no-build, no-ship investigation cycle is legitimate and stays unconstrained.
	if !planRuns(out, "ship") {
		return out, clamps
	}

	for _, phase := range floor {
		if phase == "tdd" {
			if tddPinned(in) {
				force("tdd", "ship-requires-tdd")
			}
			continue
		}
		force(phase, "ship-requires-"+phase)
	}
	return out, clamps
}

// DropUnknownPhaseRule is the clamp rule recorded when the floor removes an entry outside the known-phase set.
const DropUnknownPhaseRule = "drop-unknown-phase"

// DropUnavailablePhaseRule is the clamp rule recorded when the floor removes an entry whose persona doc is absent.
const DropUnavailablePhaseRule = "drop-unavailable-phase"

// dropUnknownPhases returns a new entry slice without unavailable or unknown phases, one Clamp per
// removal. It removes rather than sets Run=false because dispatch keys off an entry's presence.
func dropUnknownPhases(in RouteInput, plan *PhasePlan) ([]PhasePlanEntry, []Clamp) {
	known := knownPhaseSet(in, plan)
	entries := make([]PhasePlanEntry, 0, len(plan.Entries))
	var clamps []Clamp
	for _, e := range plan.Entries {
		proposed := e.Phase + "=skip"
		if e.Run {
			proposed = e.Phase + "=run"
		}
		if slices.Contains(in.UnavailablePhases, e.Phase) {
			clamps = append(clamps, Clamp{Phase: e.Phase, Rule: DropUnavailablePhaseRule, Proposed: proposed, Forced: e.Phase + "=drop"})
			continue
		}
		if _, ok := known[e.Phase]; ok {
			entries = append(entries, e)
			continue
		}
		clamps = append(clamps, Clamp{
			Phase:    e.Phase,
			Rule:     DropUnknownPhaseRule,
			Proposed: proposed,
			Forced:   e.Phase + "=drop",
		})
	}
	return entries, clamps
}

// planRuns reports whether the plan runs phase; an absent phase does not run.
func planRuns(plan *PhasePlan, phase string) bool {
	for _, e := range plan.Entries {
		if e.Phase == phase {
			return e.Run
		}
	}
	return false
}

// ensureRun sets phase's entry to run, appending one when the phase is absent.
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
		Justification: "floor: integrity floor requires " + phase,
	})
}

// tddPinned evaluates the same conditional rule as shouldRun's TDD pin; with no rule, tdd stays pinned.
func tddPinned(in RouteInput) bool {
	if rule, ok := in.Cfg.Conditional["tdd"]; ok {
		return evalCondRule(in.Signals, rule)
	}
	return true
}
