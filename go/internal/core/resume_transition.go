package core

import (
	"os"
	"path/filepath"
	"slices"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func (o *Orchestrator) resolveResumeNext(cs CycleState, current Phase, verdict string) (Phase, error) {
	if current.IsValid() {
		return o.sm.Next(current, verdict)
	}
	plan := readRoutingPlan(cs.WorkspacePath)
	keep := o.resumeFloor(cs)
	if next, ok := o.nextInOrderRun(current, plan, keep); ok {
		return holdShipFloor(cs, next), nil
	}
	if next, ok := nextFromRoutingPlan(plan, current, keep); ok {
		return holdShipFloor(cs, next), nil
	}
	return holdShipFloor(cs, o.degradeInsertedSuccessor(cs, current)), nil
}

func holdShipFloor(cs CycleState, next Phase) Phase {
	if next != PhaseShip {
		return next
	}
	for _, floor := range []Phase{PhaseBuild, PhaseAudit} {
		if !slices.Contains(cs.CompletedPhases, string(floor)) {
			return floor
		}
	}
	return next
}

func (o *Orchestrator) resumeFloor(cs CycleState) map[string]bool {
	keep := map[string]bool{string(PhaseBuild): true, router.EvaluatorFloorPhase: true, string(PhaseShip): true}
	sig, err := router.Digest(cs.WorkspacePath, cs.CompletedPhases)
	for _, p := range o.resolvedShipFloor() {
		if p != string(PhaseTDD) || err != nil || router.TddPinned(o.cfg, sig) {
			keep[p] = true
		}
	}
	for _, p := range o.cfg.Mandatory {
		keep[p] = true
	}
	return keep
}

func skipsOnResume(e router.PhasePlanEntry, keep map[string]bool) bool {
	return (!e.Run || e.ForcedBy == router.RuleShipRequiresTdd) && !keep[e.Phase]
}

func readRoutingPlan(workspace string) *router.PhasePlan {
	if workspace == "" {
		return nil
	}
	raw, err := os.ReadFile(filepath.Join(workspace, "routing-plan.json"))
	if err != nil {
		return nil
	}
	plan, err := parsePhasePlan(string(raw))
	if err != nil {
		return nil
	}
	return plan
}

func (o *Orchestrator) nextInOrderRun(current Phase, plan *router.PhasePlan, keep map[string]bool) (Phase, bool) {
	i := orderIndex(o.cfg.Order, string(current))
	if i < 0 {
		return "", false
	}
	skip := map[string]bool{}
	if plan != nil {
		for _, e := range plan.Entries {
			if skipsOnResume(e, keep) {
				skip[e.Phase] = true
			}
		}
	}
	for _, name := range o.cfg.Order[i+1:] {
		if !skip[name] {
			return Phase(name), true
		}
	}
	return PhaseEnd, true
}

func nextFromRoutingPlan(plan *router.PhasePlan, current Phase, keep map[string]bool) (Phase, bool) {
	if plan == nil {
		return "", false
	}
	found := false
	for _, e := range plan.Entries {
		if found {
			if !skipsOnResume(e, keep) {
				return Phase(e.Phase), true
			}
			continue
		}
		if e.Phase == string(current) {
			found = true
		}
	}
	if found {
		return PhaseEnd, true
	}
	return "", false
}

func (o *Orchestrator) degradeInsertedSuccessor(cs CycleState, current Phase) Phase {
	done := func(p Phase) bool { return slices.Contains(cs.CompletedPhases, string(p)) }
	archetype := o.phaseArchetype(string(current))
	if archetype == string(phasespec.RolePlan) || !done(PhaseBuild) {
		return PhaseBuild
	}
	if archetype == string(phasespec.RoleControl) {
		return PhaseShip
	}
	return PhaseAudit
}
