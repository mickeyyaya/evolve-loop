package router

import (
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

const EvaluatorFloorPhase = "audit"

func ClampPlanToFloor(in RouteInput, plan *PhasePlan) (*PhasePlan, []Clamp) {
	return ClampPlanToFloorWith(in, plan, DefaultShipFloor(), in.IntentRequired)
}

func DefaultShipFloor() []string { return []string{"tdd", "build", "audit"} }

func ClampPlanToFloorWith(in RouteInput, plan *PhasePlan, floor []string, intentRequired bool) (*PhasePlan, []Clamp) {
	if plan == nil {
		return nil, nil
	}
	floor = withEvaluatorFloor(floor)
	entries, clamps := dropUnknownPhases(in, plan)
	out := &PhasePlan{
		Entries:    entries,
		MintPhases: plan.MintPhases,
	}
	clamps = append(clamps, promoteUnifiedCommitmentTier(in, out)...)

	if intentRequired {
		forcePhase(out, &clamps, "intent", "require-intent")
	}

	if planRuns(out, "build") {
		forceBuiltWorkShipped(out, &clamps)
	}

	if !planRuns(out, "ship") {
		return out, clamps
	}

	for _, phase := range floor {
		if phase == "tdd" {
			if TddPinned(in.Cfg, in.Signals) {
				forcePhase(out, &clamps, "tdd", "ship-requires-tdd")
			}
			continue
		}
		forcePhase(out, &clamps, phase, "ship-requires-"+phase)
	}
	return out, clamps
}

func forceBuiltWorkShipped(out *PhasePlan, clamps *[]Clamp) {
	forcePhase(out, clamps, EvaluatorFloorPhase, "build-requires-"+EvaluatorFloorPhase)
	forcePhase(out, clamps, "ship", "build-requires-ship")
}

func withEvaluatorFloor(floor []string) []string {
	if slices.Contains(floor, EvaluatorFloorPhase) {
		return floor
	}
	floor = append([]string(nil), floor...)
	return append(floor, EvaluatorFloorPhase)
}

func promoteUnifiedCommitmentTier(in RouteInput, out *PhasePlan) []Clamp {
	if in.Signals.Triage.UnifiedSize == "" {
		return nil
	}
	var clamps []Clamp
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
	return clamps
}

func forcePhase(plan *PhasePlan, clamps *[]Clamp, phase, rule string) {
	if planRuns(plan, phase) {
		return
	}
	ensureRun(plan, phase)
	*clamps = append(*clamps, Clamp{
		Rule:     rule,
		Proposed: phase + "=skip",
		Forced:   phase + "=run",
	})
}

const DropUnknownPhaseRule = "drop-unknown-phase"

const DropUnavailablePhaseRule = "drop-unavailable-phase"

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

func planRuns(plan *PhasePlan, phase string) bool {
	for _, e := range plan.Entries {
		if e.Phase == phase {
			return e.Run
		}
	}
	return false
}

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

func TddPinned(cfg config.RoutingConfig, sig RoutingSignals) bool {
	if rule, ok := cfg.Conditional["tdd"]; ok {
		return evalCondRule(sig, rule)
	}
	return true
}
