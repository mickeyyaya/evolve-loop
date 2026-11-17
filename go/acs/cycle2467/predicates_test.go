//go:build acs

package cycle2467

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	integrationPackage  = "./internal/integration"
	orchestratorPackage = "./internal/orchestrator"
)

type namedTests struct {
	pkg   string
	names []string
}

func (n namedTests) run(t *testing.T) {
	t.Helper()
	pattern := "^(" + strings.Join(n.names, "|") + ")$"
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "-C", acsassert.GoModuleDir(t), "test", "-count=1", "-v", "-run", pattern, n.pkg)
	if code < 0 {
		t.Fatalf("go test did not start: %v", err)
	}
	out := stdout + stderr
	if code != 0 {
		t.Errorf("%s: go test exit %d\n%s", n.pkg, code, acsassert.OutputTail(out, 40))
	}
	for _, name := range n.names {
		if !strings.Contains(out, "--- PASS: "+name+" ") {
			t.Errorf("%s: %s did not run and pass", n.pkg, name)
		}
	}
}

func TestC2467_001_TheBatchTestFailsTheCombinedPrefixWhereTheThirdOfThreeDeliverablesBreaksATest(t *testing.T) {
	namedTests{integrationPackage, []string{
		"TestBatchPrefix_ThreeDeliverablesWhereTheLastBreaksATestFailTheCombinedPrefix",
	}}.run(t)
}

func TestC2467_002_TheThirdDeliverableIsEjectedWithTheDiffOfTheFirstTwoInItsBrief(t *testing.T) {
	namedTests{integrationPackage, []string{
		"TestBatchPrefix_TheEjectionOfTheThirdHoldsTheDiffOfTheFirstTwoInItsBrief",
		"TestBatchTest_ABatchEjectionHoldsTheDiffTheDeliverableMet",
	}}.run(t)
}

func TestC2467_003_ATextualConflictEjectsBeforeAnyTestOrGateRuns(t *testing.T) {
	namedTests{integrationPackage, []string{
		"TestBatchPrefix_ATextualConflictOfTheThirdEjectsBeforeAnyTestOrGateRuns",
		"TestCompose_ATextualConflictEjectsWithNoGateRun",
	}}.run(t)
}

func TestC2467_004_AnEnforceEjectionReachesTheOrchestratorWhichKeepsItsCellsHeldUntilTheLandingOrTheGiveUp(t *testing.T) {
	namedTests{integrationPackage, []string{
		"TestEnforceActor_AnEjectionPublishesTheEjectedEventWithTheBriefFileOfItsLane",
	}}.run(t)
	namedTests{orchestratorPackage, []string{
		"TestOversight_AnEnforceEjectionBriefsItsLaneAndKeepsItsCellsHeldUntilTheLandingFreesThem",
		"TestOversight_AnEjectedLaneThatGivesUpFreesItsCells",
	}}.run(t)
}
