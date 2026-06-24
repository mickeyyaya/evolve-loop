package router_test

// Pure-kernel scenario catalog for the ADR-0024 §1 conditional integrity floor
// ACTIVATION (PR-5 live-wiring). Proves the clamped whole-cycle plan drives
// run/skip at Stage>=Advisory, that the configurable mandatory set is the
// never-skip floor, and that the non-configurable integrity floor (ship⇒build∧
// audit∧tdd) backstops a plan that ships without the chain. Built entirely from
// the configurable routingtest framework. See internal/routingtest.

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	. "github.com/mickeyyaya/evolve-loop/go/internal/routingtest"
)

// TestFloorActivationKernel runs the explicit single-decision floor catalog.
func TestFloorActivationKernel(t *testing.T) {
	RunAll(t, floorActivationCatalog())
}

func floorActivationCatalog() []ScenarioSpec {
	return []ScenarioSpec{
		// Advisory: the clamped plan SKIPS an optional phase the trigger would
		// insert. RedBuild(2) would fire the tester trigger; the plan omits tester
		// ⇒ skipped. The advisor now drives the optional surface (vs the legacy
		// trigger-only path, which inserts tester on acs_red>0).
		Scenario("advisory plan skips tester the trigger would insert",
			Pure(), Advisory(), At("build"), Done("scout", "tdd", "build"), RedBuild(2),
			AgentPlan(PlanRun("scout"), PlanRun("tdd"), PlanRun("build"), PlanRun("audit"), PlanRun("ship")),
			ExpectNext("audit"), ExpectSkips("tester")),

		// Advisory: the clamped plan INSERTS an optional phase the trigger would
		// skip. GreenBuild ⇒ no trigger; the plan runs tester ⇒ inserted.
		Scenario("advisory plan inserts tester the trigger would skip",
			Pure(), Advisory(), At("build"), Done("scout", "tdd", "build"), GreenBuild(),
			AgentPlan(PlanRun("scout"), PlanRun("tdd"), PlanRun("build"), PlanRun("tester"), PlanRun("audit"), PlanRun("ship")),
			ExpectNext("tester"), ExpectInserts("tester")),

		// Integrity floor (the safety story): tdd-only mandatory, advisor OMITS
		// audit but ships ⇒ ClampPlanToFloor forces audit into the plan, so the
		// walk runs audit before ship even though audit ∉ cfg.Mandatory.
		Scenario("floor forces audit when tdd-only-mandatory plan ships without it",
			Pure(), Advisory(), Mandatory("tdd"), At("build"), Done("scout", "tdd", "build"),
			AgentPlan(PlanRun("scout"), PlanRun("build"), PlanRun("ship")),
			ExpectNext("audit"), ExpectInvariants("ship-implies-audit-in-plan", "determinism")),

		// Config-mandatory always wins over the plan: the plan explicitly skips
		// build, but build is in the mandatory set ⇒ never-skipped (spine:build).
		Scenario("mandatory build wins over plan skip",
			Pure(), Advisory(), At("tdd"), Done("scout", "tdd"),
			AgentPlan(PlanSkip("build")),
			ExpectNext("build"), ExpectReason("spine:build"),
			ExpectInvariants("mandatory-never-skipped")),

		// No-ship investigation cycle: minimal mandatory, plan runs only scout ⇒
		// the walk legitimately ends after scout (the floor's implication has a
		// false antecedent, so nothing is forced).
		Scenario("no-ship plan ends after scout",
			Pure(), Advisory(), Mandatory("scout"), TrivialCycle(), At("scout"), Done("scout"),
			AgentPlan(PlanRun("scout")),
			ExpectNext("end")),

		// Stage gating: the SAME plan that inserts tester at Advisory is IGNORED at
		// Shadow (below the activation floor) — the legacy trigger path drives, so
		// GreenBuild skips tester. Proves the floor is dormant below Advisory.
		Scenario("shadow ignores plan (stage-gated, trigger path drives)",
			Pure(), Shadow(), At("build"), Done("scout", "tdd", "build"), GreenBuild(),
			AgentPlan(PlanRun("scout"), PlanRun("tdd"), PlanRun("build"), PlanRun("tester"), PlanRun("audit"), PlanRun("ship")),
			ExpectNext("audit"), ExpectSkips("tester")),

		// Floor beats operator EnableOff: tdd-only mandatory, the operator disabled
		// audit (EnableOff), but the advisor ships ⇒ the floor forces audit into the
		// plan and the override is RECORDED (not silent) as a clamp.
		Scenario("floor overrides operator EnableOff for the ship-chain",
			Pure(), Advisory(), Mandatory("tdd"), PhaseEnabled("audit", config.EnableOff),
			At("build"), Done("scout", "tdd", "build"),
			AgentPlan(PlanRun("scout"), PlanRun("build"), PlanRun("ship")),
			ExpectNext("audit"), ExpectClamp("floor-overrides-enable-off")),

		// Forensics: a plan-driven (non-mandatory, non-trigger) phase is reasoned
		// "plan:<phase>", distinguishing advisor-planned from trigger-inserted.
		Scenario("plan-driven phase carries a plan: reason",
			Pure(), Advisory(), At("build"), Done("scout", "tdd", "build"), GreenBuild(),
			AgentPlan(PlanRun("scout"), PlanRun("tdd"), PlanRun("build"), PlanRun("tester"), PlanRun("audit"), PlanRun("ship")),
			ExpectNext("tester"), ExpectReason("plan:tester")),
	}
}

// TestFloorAdversarialMatrix is the safety contract as a cross-product: for
// EVERY (just-completed phase × plan shape × cycle size) under a deliberately
// minimal (tdd-only) mandatory set, the CLAMPED plan threaded into the kernel
// must satisfy ship⇒audit-in-plan and never skip the mandatory tdd — no advisor
// plan, however it shapes the cycle, weakens the integrity floor.
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
