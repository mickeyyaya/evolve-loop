package core

import (
	"fmt"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

type rePlanner interface {
	RePlan(in router.RouteInput) (*router.PhasePlan, error)
}

var postScoutReplanProbe func(cr *cycleRun)

func (cr *cycleRun) postScoutReplan() {
	if postScoutReplanProbe != nil {
		postScoutReplanProbe(cr)
	}
	if cr.o.cfg.RouterReplan == config.StageOff {
		return
	}
	if cr.o.cfg.Stage < config.StageAdvisory || cr.o.cfg.Mode != config.ModeDynamicLLM {
		return
	}
	planner, ok := cr.o.planner.(rePlanner)
	if !ok {
		return
	}
	signals, err := router.Digest(cr.cs.WorkspacePath, cr.cs.CompletedPhases)
	if err != nil {
		fmt.Fprintf(cr.o.errOut(), "[orchestrator] WARN post-scout digest degraded (cycle %d): %v\n", cr.cycle, err)
	}
	if !signals.Scout.Present {
		return
	}
	in := cr.o.advisorPlanInput(cr.ctx, string(PhaseScout), signals, cr.req, cr.state, cr.cs, cr.cycle, cr.envSnap, cr.benchedCLIs)

	if !router.PlanMismatch(in, cr.clampedPlan) {
		return
	}
	maxDepth := cr.o.cfg.RePlanMaxDepth
	if maxDepth <= 0 {
		maxDepth = 1
	}
	if cr.replanDepth >= maxDepth {
		fmt.Fprintf(cr.o.errOut(), "[orchestrator] re-plan depth cap (%d) reached at cycle %d — escalating, not re-planning\n", maxDepth, cr.cycle)
		cr.recordReplanEscalation(maxDepth)
		return
	}

	raw, err := cr.observedRePlan(planner, in)
	if err != nil || raw == nil {
		if err != nil {
			fmt.Fprintf(cr.o.errOut(), "[orchestrator] WARN post-scout re-plan failed (keeping initial plan): %v\n", err)
		}
		return
	}
	cr.replanDepth++
	cr.o.recordPlanRejectionsKind(cr.ctx, cr.cycle, cr.cs, router.ValidatePlan(in, raw), fmt.Sprintf("replan-%d", cr.replanDepth))
	clamped, clamps := router.ClampPlanToFloorWith(in, raw, cr.o.resolvedShipFloor(), cr.cs.IntentRequired)
	cr.o.recordPhasePlanKind(cr.ctx, cr.cycle, cr.cs, clamped, clamps, "replan")

	if cr.o.cfg.RouterReplan == config.StageAdvisory {
		cr.clampedPlan = clamped
		cr.o.registerMintedPhases(clamped)
	}
}

func (cr *cycleRun) recordReplanEscalation(maxDepth int) {
	if err := cr.o.ledger.Append(cr.ctx, LedgerEntry{
		TS: cr.o.now().UTC().Format(time.RFC3339), Cycle: cr.cycle, Role: "orchestrator",
		Kind: "replan_escalation", ExitCode: 0,
	}); err != nil {
		fmt.Fprintf(cr.o.errOut(), "[orchestrator] WARN replan_escalation ledger append: %v\n", err)
	}
}
