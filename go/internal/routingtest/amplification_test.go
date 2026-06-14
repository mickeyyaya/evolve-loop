package routingtest

import (
	"testing"
)

func TestAmplify_AdvisoryFailuresCannotWeakenAuditOrdering(t *testing.T) {
	RunAll(t, []ScenarioSpec{
		Scenario("agent error falls back to audit after green build",
			Pure(), Advisory(), At("build"), Done("scout", "tdd", "build"), GreenBuild(),
			AgentError("build"),
			ExpectNext("audit"),
			ExpectInvariants("no-ship-before-audit", "determinism"),
		),
		Scenario("agent plan error falls back to audit after green build",
			Pure(), Advisory(), At("build"), Done("scout", "tdd", "build"), GreenBuild(),
			AgentPlanError(),
			ExpectNext("audit"),
			ExpectInvariants("no-ship-before-audit", "determinism"),
		),
	})
}
