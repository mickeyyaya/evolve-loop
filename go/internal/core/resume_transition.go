package core

import (
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// resolveResumeNext computes the successor of a just-completed phase on the
// resume path: a spine-valid current phase uses the exact static kernel
// (o.sm.Next), so resume stays byte-identical for the common case. An
// inserted (invalid) phase resolves via the run's own routing-plan.json,
// falling back to an archetype degrade when that plan is absent.
func (o *Orchestrator) resolveResumeNext(cs CycleState, current Phase, verdict string) (Phase, error) {
	if current.IsValid() {
		return o.sm.Next(current, verdict)
	}
	if next, ok := nextFromRoutingPlan(cs.WorkspacePath, current); ok {
		return next, nil
	}
	return o.degradeInsertedSuccessor(current), nil
}

// nextFromRoutingPlan returns the phase that follows current in
// <workspace>/routing-plan.json's run:true order, PhaseEnd when current is
// the plan's last entry, and ok=false when the plan is missing/unparseable
// or current is absent from it. It reuses parsePhasePlan so the wire format
// stays single-sourced.
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
		// current is in the plan but nothing runs after it — the plan is complete.
		return PhaseEnd, true
	}
	return "", false
}

// degradeInsertedSuccessor is the archetype fallback when no routing-plan.json
// is available: it routes an inserted phase to the static spine phase its
// composition archetype flows into, never skipping the audit gate.
func (o *Orchestrator) degradeInsertedSuccessor(current Phase) Phase {
	switch o.phaseArchetype(string(current)) {
	case string(phasespec.RolePlan):
		return PhaseBuild
	case string(phasespec.RoleBuild), string(phasespec.RoleEvaluate):
		return PhaseAudit
	case string(phasespec.RoleControl):
		return PhaseShip
	default:
		return PhaseAudit
	}
}
