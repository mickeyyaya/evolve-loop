package router_test

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	. "github.com/mickeyyaya/evolve-loop/go/internal/routingtest"
)

func TestFloorActivationKernel(t *testing.T) {
	RunAll(t, floorActivationCatalog())
}

func floorActivationCatalog() []ScenarioSpec {
	return []ScenarioSpec{
		Scenario("advisory plan skips tester the trigger would insert",
			Pure(), Advisory(), At("build"), Done("scout", "tdd", "build"), RedBuild(2),
			AgentPlan(PlanRun("scout"), PlanRun("tdd"), PlanRun("build"), PlanRun("audit"), PlanRun("ship")),
			ExpectNext("audit"), ExpectSkips("tester")),

		Scenario("advisory plan inserts tester the trigger would skip",
			Pure(), Advisory(), At("build"), Done("scout", "tdd", "build"), GreenBuild(),
			AgentPlan(PlanRun("scout"), PlanRun("tdd"), PlanRun("build"), PlanRun("tester"), PlanRun("audit"), PlanRun("ship")),
			ExpectNext("tester"), ExpectInserts("tester")),

		Scenario("floor forces audit when tdd-only-mandatory plan ships without it",
			Pure(), Advisory(), Mandatory("tdd"), At("build"), Done("scout", "tdd", "build"),
			AgentPlan(PlanRun("scout"), PlanRun("build"), PlanRun("ship")),
			ExpectNext("audit"), ExpectInvariants("ship-implies-audit-in-plan", "determinism")),

		Scenario("mandatory build wins over plan skip",
			Pure(), Advisory(), At("tdd"), Done("scout", "tdd"),
			AgentPlan(PlanSkip("build")),
			ExpectNext("build"), ExpectReason("spine:build"),
			ExpectInvariants("mandatory-never-skipped")),

		Scenario("no-ship plan ends after scout",
			Pure(), Advisory(), Mandatory("scout"), TrivialCycle(), At("scout"), Done("scout"),
			AgentPlan(PlanRun("scout")),
			ExpectNext("end")),

		Scenario("shadow ignores plan (stage-gated, trigger path drives)",
			Pure(), Shadow(), At("build"), Done("scout", "tdd", "build"), GreenBuild(),
			AgentPlan(PlanRun("scout"), PlanRun("tdd"), PlanRun("build"), PlanRun("tester"), PlanRun("audit"), PlanRun("ship")),
			ExpectNext("audit"), ExpectSkips("tester")),

		Scenario("floor overrides operator EnableOff for the ship-chain",
			Pure(), Advisory(), Mandatory("tdd"), PhaseEnabled("audit", config.EnableOff),
			At("build"), Done("scout", "tdd", "build"),
			AgentPlan(PlanRun("scout"), PlanRun("build"), PlanRun("ship")),
			ExpectNext("audit"), ExpectClamp("floor-overrides-enable-off")),

		Scenario("plan-driven phase carries a plan: reason",
			Pure(), Advisory(), At("build"), Done("scout", "tdd", "build"), GreenBuild(),
			AgentPlan(PlanRun("scout"), PlanRun("tdd"), PlanRun("build"), PlanRun("tester"), PlanRun("audit"), PlanRun("ship")),
			ExpectNext("tester"), ExpectReason("plan:tester")),
	}
}

func TestFloorAdversarialMatrix(t *testing.T) {
	specs := Matrix(
		[]Brick{Pure(), Advisory(), Mandatory("tdd"),
			ExpectInvariants("ship-implies-audit-in-plan", "mandatory-never-skipped", "determinism")},
		Dim("at",
			V("scout", At("scout"), Done("scout")),
			V("build", At("build"), Done("scout", "tdd", "build")),
		),
		Dim("plan",
			V("ship-no-audit", AgentPlan(PlanRun("scout"), PlanRun("build"), PlanRun("ship"))),
			V("ship-full", AgentPlan(PlanRun("scout"), PlanRun("tdd"), PlanRun("build"), PlanRun("audit"), PlanRun("ship"))),
			V("no-ship", AgentPlan(PlanRun("scout"))),
		),
		Dim("size", V("trivial", TrivialCycle()), V("medium", MediumCycle())),
	)
	RunAll(t, specs)
}
