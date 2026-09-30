package core

import (
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

const CodePlanPhaseGated signalcenter.Code = "ORCHESTRATOR_PLAN_PHASE_GATED"

func init() {
	signalcenter.RegisterCode(signalcenter.ModuleOrchestrator, CodePlanPhaseGated,
		"the phase registry removed a phase the advisor's plan runs, at that phase's turn in the walk: its declared skip_when fired (rule skip-when-gates-plan, e.g. a bugfix phase on a deliverable_kind=document cycle) or its insert_when, its whole admission rule, did not fire (rule insert-when-gates-plan); fields.phase, rule, next_phase; the walk continues and routing-decision-<n>.json keeps the clamp")
}

func (o *Orchestrator) emitRegistryGatedPhases(cycle int, cs CycleState, dec router.RouterDecision) {
	for _, c := range dec.Clamps {
		if c.Rule != router.RuleSkipWhenGatesPlan && c.Rule != router.RuleInsertWhenGatesPlan {
			continue
		}
		o.signals.Emit(signalcenter.Event{
			Cycle: cycle, RunID: cs.RunID, Phase: c.Phase, Module: signalcenter.ModuleOrchestrator,
			Origin: "Orchestrator.recordRoutingDecision", Kind: signalcenter.KindAdvisorWarning, Severity: signalcenter.SeverityWarn,
			Code:   CodePlanPhaseGated,
			Reason: "the phase registry removed planned phase " + c.Phase + " (" + c.Rule + "); next is " + dec.NextPhase,
			Fields: map[string]string{"phase": c.Phase, "rule": c.Rule, "next_phase": dec.NextPhase},
		})
	}
}
