package routingtest

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/router"
)

func TestInvariant_ProposalNeverWeakens(t *testing.T) {
	RunAll(t, []ScenarioSpec{
		Scenario("hostile proposal cannot jump past audit",
			Pure(), At("build"), Done("scout", "tdd", "build"), GreenBuild(),
			Agent("build", "ship"),
			ExpectInvariants("proposal-never-weakens"),
		),
	})
}

func TestInvariant_MandatoryNeverSkipped(t *testing.T) {
	RunAll(t, []ScenarioSpec{
		Scenario("mandatory off clamp stays out of SkipPhases",
			Pure(), At("start"), Mandatory("scout", "build"), PhaseEnabled("scout", config.EnableOff),
			ExpectInvariants("mandatory-never-skipped"),
		),
	})
}

func TestInvariant_NoShipBeforeAudit(t *testing.T) {
	RunAll(t, []ScenarioSpec{
		Scenario("green build routes to audit before ship",
			Pure(), At("build"), Done("scout", "tdd", "build"), GreenBuild(),
			ExpectInvariants("no-ship-before-audit"),
		),
	})
}

func TestInvariant_TDDPinNontrivial(t *testing.T) {
	RunAll(t, []ScenarioSpec{
		Scenario("nontrivial scout keeps tdd pinned",
			Pure(), At("scout"), Done("scout"), MediumCycle(), PhaseEnabled("tdd", config.EnableOff),
			ExpectInvariants("tdd-pin-nontrivial"),
		),
	})
}

func TestInvariant_InsertLECap(t *testing.T) {
	RunAll(t, []ScenarioSpec{
		Scenario("single tester insert respects cap",
			Pure(), At("build"), Done("scout", "tdd", "build"), RedBuild(1), MaxInserts(1),
			ExpectInvariants("insert-le-cap"),
		),
	})
}

func TestInvariant_BudgetZeroNoContentInsert(t *testing.T) {
	RunAll(t, []ScenarioSpec{
		Scenario("exhausted budget blocks trigger-driven tester insert",
			Pure(), At("build"), Done("scout", "tdd", "build"), RedBuild(2), Budget(-1),
			ExpectInvariants("budget-zero-no-content-insert"),
		),
	})
}

func TestInvariant_Determinism(t *testing.T) {
	RunAll(t, []ScenarioSpec{
		Scenario("same input produces same decision",
			Pure(), At("build"), Done("scout", "tdd", "build"), RedBuild(2),
			ExpectInvariants("determinism"),
		),
	})
}

func TestInvariant_ShipImpliesAuditInPlan(t *testing.T) {
	RunAll(t, []ScenarioSpec{
		Scenario("floor completes a ship plan before invariant assertion",
			Pure(), Advisory(), Mandatory("scout"), At("scout"), Done("scout"), SmallCycle(),
			AgentPlan(PlanRun("scout"), PlanRun("ship")),
			ExpectInvariants("ship-implies-audit-in-plan"),
		),
	})
}

func TestInvariant_DuplicatePhaseTolerated(t *testing.T) {
	RunAll(t, []ScenarioSpec{
		Scenario("duplicate plan entries still yield one deterministic walk",
			Pure(), Advisory(), Mandatory("scout"), At("scout"), Done("scout"), SmallCycle(),
			AgentPlan(PlanRun("scout"), PlanRun("ship"), PlanSkip("ship"), PlanRun("ship")),
			ExpectNext("tdd"),
			ExpectInvariants("ship-implies-audit-in-plan", "determinism"),
		),
	})
}

func TestInvariant_NoDuplicatePhaseEnforcesUniqueness(t *testing.T) {
	t.Run("unique_plan_is_silent", func(t *testing.T) {
		RunAll(t, []ScenarioSpec{
			Scenario("unique plan phases satisfy invariant",
				Pure(), Advisory(), Mandatory("scout"), At("scout"), Done("scout"), SmallCycle(),
				AgentPlan(PlanRun("scout"), PlanRun("tdd"), PlanRun("build"), PlanRun("audit"), PlanRun("ship")),
				ExpectInvariants("no-duplicate-phase"),
			),
		})
	})

	t.Run("duplicate_plan_is_detected", func(t *testing.T) {
		duplicates := duplicatePlanPhases(&router.PhasePlan{
			Entries: []router.PhasePlanEntry{
				PlanRun("scout"),
				PlanRun("ship"),
				PlanSkip("ship"),
			},
		})
		if len(duplicates) != 1 || duplicates[0] != "ship" {
			t.Fatalf("duplicatePlanPhases() = %v, want [ship]", duplicates)
		}
	})
}
