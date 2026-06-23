package core_test

// End-to-end orchestrator scenario catalog for the ADR-0024 §1 conditional
// integrity floor ACTIVATION (PR-5 live-wiring): the orchestrator computes the
// upfront whole-cycle plan, clamps it to the floor, and lets it DRIVE phase
// selection at Stage>=Advisory. Proves the advisor drives the non-mandatory
// surface, the floor forces the ship-chain regardless of how small the operator
// makes cfg.Mandatory, and a planner failure degrades cleanly to the static
// spine. Built on the configurable routingtest framework.

import (
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
	. "github.com/mickeyyaya/evolveloop/go/internal/routingtest"
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
		// Advisory DRIVES + tdd-only mandatory: the advisor's plan runs the spine;
		// triage + build-planner (absent from the plan) are skipped. The floor
		// activates at Advisory, not only Enforce.
		Scenario("advisory tdd-only mandatory: plan drives spine, optionals skipped",
			Cycle(), Advisory(), Mandatory("tdd"), MediumCycle(),
			AgentPlan(PlanRun("scout"), PlanRun("tdd"), PlanRun("build"), PlanRun("audit"), PlanRun("ship")),
			ExpectPhases(scout, tdd, build, audit, ship),
			ExpectAbsent(triage, planner)),

		// THE safety scenario: tdd-only mandatory, the advisor omits build+audit
		// but ships ⇒ ClampPlanToFloor forces them; build+audit RUN before ship
		// despite being absent from both the advisor's plan AND cfg.Mandatory.
		Scenario("floor forces build+audit when advisor ships without them",
			Cycle(), Advisory(), Mandatory("tdd"), MediumCycle(),
			AgentPlan(PlanRun("scout"), PlanRun("ship")),
			ExpectPhases(scout, tdd, build, audit, ship)),

		// Enforce mode safety: the advisor omits build+audit but ships, and under
		// Enforce() the floor forces build+audit to run before ship.
		Scenario("enforce: advisor ships without build+audit forces floor",
			Cycle(), Enforce(), Mandatory("tdd"), MediumCycle(),
			AgentPlan(PlanRun("scout"), PlanRun("ship")),
			ExpectPhases(scout, tdd, build, audit, ship)),

		// Fail-safe-to-floor: a planner error ⇒ nil plan ⇒ the configurable spine
		// drives via the trigger path (the cycle still ships safely, never crashes).
		Scenario("planner error degrades to static spine",
			Cycle(), Advisory(), MediumCycle(), AgentPlanError(),
			ExpectPhases(scout, tdd, build, audit, ship)),

		// Stage:Off ignores a scripted plan entirely — byte-identical legacy spine,
		// no upfront plan call, no routing forensics.
		Scenario("stage-off ignores plan (byte-identical legacy spine)",
			Cycle(), Off(),
			AgentPlan(PlanRun("scout"), PlanRun("ship")),
			ExpectPhases(scout, triage, tdd, planner, build, audit, ship)),

		// Hybrid cadence (ADR-0024 §2): with the upfront plan driving, the
		// per-transition Proposer fires ONLY at branch transitions (post-build,
		// post-audit) — not at start/scout/tdd/ship. Proves the double-spend is
		// removed without changing the routed sequence.
		Scenario("hybrid cadence: Propose fires only at branch transitions under a plan",
			Cycle(), Advisory(), Mandatory("tdd"), MediumCycle(),
			AgentPlan(PlanRun("scout"), PlanRun("tdd"), PlanRun("build"), PlanRun("audit"), PlanRun("ship")),
			ExpectProposeAt("build", "audit"),
			ExpectPhases(scout, tdd, build, audit, ship)),
	}
}
