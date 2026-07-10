package core

import (
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

// resume_transition.go — cycle-637 (inbox resume-dynamic-phase-transition, 0.93).
// Rehydrates the transition kernel on `evolve loop --resume` so a resumed cycle
// can transition OUT of an advisor-inserted phase (bug-reproduction,
// fault-localization — real catalog phases the dynamic router splices onto
// bugfix cycles, none spine-valid). The static kernel (o.sm.Next) returns
// "core: invalid phase: <phase>" for them, so the resumed cycle-635 died and its
// bare error escaped the ADR-0044 C1 chokepoint as FAILED_UNEXPLAINED.

// resolveResumeNext computes the successor of a just-completed phase on the
// RESUME path. A spine-valid current keeps the EXACT static kernel (o.sm.Next) —
// verdict branches and all — so resume stays byte-identical for the common case.
// Only an inserted (invalid) phase takes the rehydration path: first the run's
// own routing-plan.json (the SAME effective sequence the original run followed),
// then a graceful archetype degrade when that plan is absent.
func (o *Orchestrator) resolveResumeNext(cs CycleState, current Phase, verdict string) (Phase, error) {
	if current.IsValid() {
		return o.sm.Next(current, verdict)
	}
	if next, ok := nextFromRoutingPlan(cs.WorkspacePath, current); ok {
		return next, nil
	}
	return o.degradeInsertedSuccessor(current), nil
}

// nextFromRoutingPlan reads <workspace>/routing-plan.json — the advisor's
// whole-cycle plan, the SAME artifact the original run produced and
// parsePhasePlan reads — and returns the phase that follows `current` in the
// planned run order: the next entry marked run:true after current's own entry.
// When current is the last run:true entry the cycle is planned-complete
// (PhaseEnd). ok is false when the plan file is missing/unparseable or current is
// absent from it — the caller then degrades. Reusing parsePhasePlan keeps the
// plan wire format single-sourced.
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

// degradeInsertedSuccessor is the archetype fallback when no routing-plan.json is
// available (deleted, or a pre-plan checkpoint): the inserted phase can no longer
// be positioned exactly, so it is treated as its composition archetype and routed
// to the static spine phase that archetype flows INTO — never skipping the audit
// gate. A plan-archetype bugfix insert (bug-reproduction / fault-localization)
// resolves to build (write the fix), mirroring the real bugfix flow; any unknown
// archetype defaults to audit, the safe integrity funnel.
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
