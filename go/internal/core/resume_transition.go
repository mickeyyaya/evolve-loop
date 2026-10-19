package core

import (
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

func (o *Orchestrator) resolveResumeNext(cs CycleState, current Phase, verdict string) (Phase, error) {
	if current.IsValid() {
		return o.sm.Next(current, verdict)
	}
	if next, ok := nextFromRoutingPlan(cs.WorkspacePath, current); ok {
		return next, nil
	}
	return o.degradeInsertedSuccessor(current), nil
}

func nextFromRoutingPlan(workspace string, current Phase) (Phase, bool) {
	if workspace == "" {
		return "", false
	}
	raw, err := os.ReadFile(filepath.Join(workspace, "routing-plan.json"))
	if err != nil {
		return "", false
	}
	plan, err := parsePhasePlan(string(raw))
	if err != nil {
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
