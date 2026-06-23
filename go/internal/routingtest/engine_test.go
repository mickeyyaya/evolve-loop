package routingtest

import (
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
)

func TestEngine_RunAllExecutesPureAndCycleSpecs(t *testing.T) {
	RunAll(t, []ScenarioSpec{
		Scenario("pure/default config routes start to scout",
			Pure(), At("start"),
			ExpectNext("scout"),
			ExpectInvariants("determinism"),
		),
		Scenario("pure/advisory plan records advisor justification",
			Pure(), Advisory(), At("build"), Done("scout", "tdd", "build"), GreenBuild(),
			AgentJustified("build", "ship", "skip audit anyway"),
			ExpectNext("audit"),
			ExpectClamp("llm-proposal-clamped"),
			ExpectJustification("skip audit anyway"),
			ExpectInvariants("proposal-never-weakens", "no-ship-before-audit"),
		),
		Scenario("cycle surface emits routing decisions",
			Cycle(), Advisory(), SmallCycle(), GreenBuild(), AuditIs("PASS"),
			Agent("scout", "ship"),
			PhaseVerdict(core.PhaseScout, "PASS"),
			PhaseVerdict(core.PhaseTDD, "PASS"),
			PhaseVerdict(core.PhaseBuild, "PASS"),
			PhaseVerdict(core.PhaseAudit, "PASS"),
			ExpectRoutingLedger(1),
			ExpectDecisionClamp("llm-proposal-clamped"),
		),
	})
}
