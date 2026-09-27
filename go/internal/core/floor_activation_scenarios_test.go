package core_test

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	. "github.com/mickeyyaya/evolve-loop/go/internal/routingtest"
)

func TestFloorActivationCycle(t *testing.T) {
	t.Parallel()
	RunAll(t, floorActivationCycleCatalog())
}

func floorActivationCycleCatalog() []ScenarioSpec {
	const (
		scout   = core.PhaseScout
		triage  = core.PhaseTriage
		tdd     = core.PhaseTDD
		planner = core.PhaseBuildPlanner
		build   = core.PhaseBuild
		audit   = core.PhaseAudit
		ship    = core.PhaseShip
	)
	return []ScenarioSpec{
		// The floor activates at Advisory, not only Enforce.
		Scenario("advisory tdd-only mandatory: plan drives spine, optionals skipped",
			Cycle(), Advisory(), Mandatory("tdd"), MediumCycle(),
			AgentPlan(PlanRun("scout"), PlanRun("tdd"), PlanRun("build"), PlanRun("audit"), PlanRun("ship")),
			ExpectPhases(scout, tdd, build, audit, ship),
			ExpectAbsent(triage, planner)),

		// ClampPlanToFloor forces build+audit before ship even though both are
		// absent from the advisor's plan and cfg.Mandatory.
		Scenario("floor forces build+audit when advisor ships without them",
			Cycle(), Advisory(), Mandatory("tdd"), MediumCycle(),
			AgentPlan(PlanRun("scout"), PlanRun("ship")),
			ExpectPhases(scout, tdd, build, audit, ship)),

		Scenario("enforce: advisor ships without build+audit forces floor",
			Cycle(), Enforce(), Mandatory("tdd"), MediumCycle(),
			AgentPlan(PlanRun("scout"), PlanRun("ship")),
			ExpectPhases(scout, tdd, build, audit, ship)),

		// A nil plan falls back to the configurable spine via the trigger path.
		Scenario("planner error degrades to static spine",
			Cycle(), Advisory(), MediumCycle(), AgentPlanError(),
			ExpectPhases(scout, tdd, build, audit, ship)),

		// Off: no upfront plan call and no routing forensics.
		Scenario("stage-off ignores plan (byte-identical legacy spine)",
			Cycle(), Off(),
			AgentPlan(PlanRun("scout"), PlanRun("ship")),
			ExpectPhases(scout, triage, tdd, planner, build, audit, ship)),

		// With the upfront plan driving, the per-transition Proposer fires only at
		// branch transitions (post-build, post-audit), not at every phase, so the
		// plan and the proposer never double-spend a routing decision.
		Scenario("hybrid cadence: Propose fires only at branch transitions under a plan",
			Cycle(), Advisory(), Mandatory("tdd"), MediumCycle(),
			AgentPlan(PlanRun("scout"), PlanRun("tdd"), PlanRun("build"), PlanRun("audit"), PlanRun("ship")),
			ExpectProposeAt("build", "audit"),
			ExpectPhases(scout, tdd, build, audit, ship)),
	}
}
