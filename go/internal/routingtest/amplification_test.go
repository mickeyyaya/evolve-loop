package routingtest

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
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

func TestAmplify_ExactZeroBudgetDoesNotInsertTester(t *testing.T) {
	RunAll(t, []ScenarioSpec{
		Scenario("exact zero budget blocks trigger-driven tester insert",
			Pure(), At("build"), Done("scout", "tdd", "build"), RedBuild(2), Budget(0), MaxInserts(1),
			ExpectAbsent(core.Phase("tester")),
			ExpectInvariants("budget-zero-no-content-insert", "insert-le-cap"),
		),
	})
}
