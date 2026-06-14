package core_test

// End-to-end orchestrator scenario catalog for dynamic phase-routing, built on
// the configurable routingtest framework. Covers distinct driven phase
// combinations across stages plus simulated-agent decisions. The 4 scenarios
// already in orchestrator_routing_test.go (StageOff-no-forensics, shadow-tester,
// enforce-trivial, spine-warn) are intentionally NOT duplicated here.

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	. "github.com/mickeyyaya/evolve-loop/go/internal/routingtest"
)

func TestCycleScenarios(t *testing.T) {
	t.Parallel()
	RunAll(t, cycleCatalog())
}

func cycleCatalog() []ScenarioSpec {
	const (
		intent  = core.PhaseIntent
		scout   = core.PhaseScout
		triage  = core.PhaseTriage
		tdd     = core.PhaseTDD
		planner = core.PhaseBuildPlanner
		build   = core.PhaseBuild
		audit   = core.PhaseAudit
		ship    = core.PhaseShip
		retro   = core.PhaseRetro
	)
	return []ScenarioSpec{
		// --- Shadow: router logs decisions, static path unchanged ---
		Scenario("shadow trivial logs skip but static runs full middle",
			Cycle(), Shadow(), TrivialCycle(),
			ExpectPhases(scout, triage, tdd, planner, build, audit, ship),
			ExpectRoutingLedger(1)),

		// --- Enforce: router drives the drivable spine ---
		Scenario("enforce non-trivial skips no-trigger optionals (triage, build-planner)",
			Cycle(), Enforce(), MediumCycle(),
			ExpectPhases(scout, tdd, build, audit, ship),
			ExpectAbsent(triage, planner)),
		Scenario("enforce tester insert is shadow-only (decision logged, never driven)",
			Cycle(), Enforce(), MediumCycle(), RedBuild(2),
			ExpectDecisionInsert("tester"),
			ExpectPhases(scout, tdd, build, audit, ship)),
		Scenario("enforce severity-HIGH tester insert shadow-only",
			Cycle(), Enforce(), MediumCycle(), Severity("HIGH"), SeverityTrigger(),
			ExpectDecisionInsert("tester"),
			ExpectPhases(scout, tdd, build, audit, ship)),
		Scenario("enforce triage forced-off",
			Cycle(), Enforce(), MediumCycle(), TriageOff(),
			ExpectAbsent(triage)),

		// --- Intent gate ---
		Scenario("intent-required runs intent first",
			Cycle(), Shadow(), IntentRequired(),
			ExpectPhases(intent, scout, triage, tdd, planner, build, audit, ship)),

		// --- Retro arcs (SM + decideAfterRetro driven; routing-on coexists) ---
		Scenario("retro PASS recovers to ship",
			Cycle(), Enforce(), MediumCycle(),
			PhaseVerdict(audit, "FAIL"), PhaseVerdict(retro, "PASS"),
			ExpectRetro("retro-recovered: ship")),
		Scenario("retro FAIL no history proceeds to end",
			Cycle(), Enforce(), MediumCycle(),
			PhaseVerdict(audit, "FAIL"), PhaseVerdict(retro, "FAIL"),
			ExpectRetro("proceed:")),
		Scenario("retro FAIL recurring audit fails fluent proceed",
			Cycle(), Enforce(), MediumCycle(),
			PhaseVerdict(audit, "FAIL"), PhaseVerdict(retro, "FAIL"),
			SeedFailure("code-audit-fail", 2),
			ExpectRetro("proceed:")),

		// --- Simulated agent end-to-end ---
		Scenario("agent legal-divergent adopted (scout->build on trivial)",
			Cycle(), Enforce(), TrivialCycle(), Agent("scout", "build"),
			ExpectPhases(scout, build, audit, ship), ExpectAbsent(tdd)),
		Scenario("agent illegal ship-from-build clamped to audit",
			Cycle(), Enforce(), MediumCycle(), GreenBuild(), Agent("build", "ship"),
			ExpectPhases(scout, tdd, build, audit, ship),
			ExpectDecisionClamp("llm-proposal-clamped")),
		Scenario("agent error degrades to static",
			Cycle(), Enforce(), MediumCycle(), AgentError("scout"),
			ExpectPhases(scout, tdd, build, audit, ship)),
	}
}
