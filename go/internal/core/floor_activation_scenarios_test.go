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
		Scenario("advisory tdd-only mandatory: plan drives spine, optionals skipped",
			Cycle(), Advisory(), Mandatory("tdd"), MediumCycle(),
			AgentPlan(PlanRun("scout"), PlanRun("tdd"), PlanRun("build"), PlanRun("audit"), PlanRun("ship")),
			ExpectPhases(scout, tdd, build, audit, ship),
			ExpectAbsent(triage, planner)),

		Scenario("floor forces build+audit when advisor ships without them",
			Cycle(), Advisory(), Mandatory("tdd"), MediumCycle(),
			AgentPlan(PlanRun("scout"), PlanRun("ship")),
			ExpectPhases(scout, tdd, build, audit, ship)),

		Scenario("enforce: advisor ships without build+audit forces floor",
			Cycle(), Enforce(), Mandatory("tdd"), MediumCycle(),
			AgentPlan(PlanRun("scout"), PlanRun("ship")),
			ExpectPhases(scout, tdd, build, audit, ship)),

		Scenario("planner error degrades to static spine",
			Cycle(), Advisory(), MediumCycle(), AgentPlanError(),
			ExpectPhases(scout, tdd, build, audit, ship)),

		Scenario("stage-off ignores plan (byte-identical legacy spine)",
			Cycle(), Off(),
			AgentPlan(PlanRun("scout"), PlanRun("ship")),
			ExpectPhases(scout, triage, tdd, planner, build, audit, ship)),

		Scenario("hybrid cadence: Propose fires only at branch transitions under a plan",
			Cycle(), Advisory(), Mandatory("tdd"), MediumCycle(),
			AgentPlan(PlanRun("scout"), PlanRun("tdd"), PlanRun("build"), PlanRun("audit"), PlanRun("ship")),
			ExpectProposeAt("build", "audit"),
			ExpectPhases(scout, tdd, build, audit, ship)),
	}
}
