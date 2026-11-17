//go:build acs

package cycle2464

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	cmdPackage      = "./cmd/evolve/"
	loopwavePackage = "./internal/loopwave"
)

func requirePass(t *testing.T, pkg string, names ...string) {
	t.Helper()
	pattern := "^(" + strings.Join(names, "|") + ")$"
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "-C", acsassert.GoModuleDir(t), "test", "-count=1", "-v", "-run", pattern, pkg)
	if code < 0 {
		t.Fatalf("go test did not start: %v", err)
	}
	out := stdout + stderr
	if code != 0 {
		t.Errorf("%s: go test exit %d\n%s", pkg, code, acsassert.OutputTail(out, 40))
	}
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name+" ") {
			t.Errorf("%s: %s did not run and pass", pkg, name)
		}
	}
}

func TestC2464_001_AHigherScoredItemTakesTheLowestUnstartedSlot(t *testing.T) {
	requirePass(t, loopwavePackage,
		"TestPreempt_AHigherScoredItemTakesTheLowestUnstartedSlot",
		"TestPreempt_OnlyTheLowestScoredSlotIsReplacedWhateverItsPlace",
		"TestPreempt_AnItemTheLanesCannotTakeNeverPreempts",
		"TestPreempt_ABacklogWithNothingOutsideThePlanLeavesPlanAndLedgerUntouched",
		"TestPreempt_AnEmptyPriorPlanDoesNotPanicOrPreempt")
	requirePass(t, cmdPackage, "TestPreempt_ProductionWavePlanFnSwapsTheLowestSlotAndLedgersIt")
}

func TestC2464_002_ARunningLaneIsNeverDisplaced(t *testing.T) {
	requirePass(t, loopwavePackage, "TestPreempt_ARunningLaneIsNeverDisplaced")
}

func TestC2464_003_AnAdoptedContinuationIsNeverDisplaced(t *testing.T) {
	requirePass(t, loopwavePackage,
		"TestPreempt_AnAdoptedContinuationIsNeverDisplaced",
		"TestPreempt_APlanOfOnlyAdoptedContinuationsIsNeverPreempted")
}

func TestC2464_004_TheLedgerRecordsPreemptedOldForNewWithBothScores(t *testing.T) {
	requirePass(t, loopwavePackage,
		"TestPreempt_TheLedgerRecordsTheOldAndTheNewItemWithBothScores",
		"TestPreempt_APreviewPlansTheSwapButWritesNoLedgerLine")
	requirePass(t, cmdPackage, "TestPreempt_ProductionWavePlanFnSwapsTheLowestSlotAndLedgersIt")
}

func TestC2464_005_ALeadBelowThePreemptMarginDoesNotPreempt(t *testing.T) {
	requirePass(t, loopwavePackage,
		"TestPreempt_ALeadBelowThePreemptMarginDoesNotPreempt",
		"TestPreempt_TheMarginComesFromThePolicyNotAConstant")
}
