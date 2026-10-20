package core

import (
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func (o *Orchestrator) resolveResumeNext(cs CycleState, current Phase, verdict string) (Phase, error) {
	if current.IsValid() {
		return o.sm.Next(current, verdict)
	}
	plan := readRoutingPlan(cs.WorkspacePath)
	if next, ok := o.nextInOrderRun(current, plan); ok {
		return next, nil
	}
	if next, ok := nextFromRoutingPlan(plan, current); ok {
		return next, nil
	}
	return o.degradeInsertedSuccessor(current), nil
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

func (o *Orchestrator) nextInOrderRun(current Phase, plan *router.PhasePlan) (Phase, bool) {
	i := orderIndex(o.cfg.Order, string(current))
	if i < 0 {
		return "", false
	}
	skip := map[string]bool{}
	if plan != nil {
		for _, e := range plan.Entries {
			if !e.Run {
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

func nextFromRoutingPlan(plan *router.PhasePlan, current Phase) (Phase, bool) {
	if plan == nil {
		return "", false
	}
	found := false
	for _, e := range plan.Entries {
		if found {
			if e.Run {
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

func (o *Orchestrator) degradeInsertedSuccessor(current Phase) Phase {
	switch o.phaseArchetype(string(current)) {
	case string(phasespec.RolePlan):
		return PhaseBuild
	case string(phasespec.RoleControl):
		return PhaseShip
	default:
		return PhaseAudit
	}
}
