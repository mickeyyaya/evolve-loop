//go:build acs

package cycle2555

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	carryoverPkg = "./internal/core/carryover"
	corePkg      = "./internal/core"
)

func goTestPasses(t *testing.T, pkg, name string) {
	t.Helper()
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     filepath.Join(acsassert.RepoRoot(t), "go"),
		Package: pkg,
		Pattern: "^" + name + "$",
		Names:   []string{name},
	})
}

func TestC2555_001_ATodoWithNoDocumentOnMainStaysInTheLifecycleStep(t *testing.T) {
	goTestPasses(t, carryoverPkg, "TestJudgmentRetire_ATodoWithNoDocumentStays")
}

func TestC2555_002_ACloseoutAtEnforceKeepsAJudgmentTodoWhoseDocumentIsNotOnMain(t *testing.T) {
	goTestPasses(t, corePkg, "TestRunCycle_AtEnforceAJudgmentTodoWithNoDocumentOnMainStays")
}

func TestC2555_003_ATodoWithItsDocumentOnMainLeavesInTheLifecycleStep(t *testing.T) {
	goTestPasses(t, carryoverPkg, "TestJudgmentRetire_ATodoWithItsDocumentLeavesAndTheOthersKeepTheirOrder")
}

func TestC2555_004_ACloseoutAtEnforceRetiresTheJudgmentTodoWhoseDocumentIsOnMain(t *testing.T) {
	goTestPasses(t, corePkg, "TestRunCycle_AtEnforceAJudgmentTodoWithItsDocumentOnMainLeavesAndTheRestStay")
}

func TestC2555_005_ASecondRunOfTheLifecycleStepChangesNothing(t *testing.T) {
	goTestPasses(t, carryoverPkg, "TestJudgmentRetire_ASecondRunChangesNothing")
}

func TestC2555_006_ASecondCloseoutAtEnforceChangesNothingInTheCarryoverLog(t *testing.T) {
	goTestPasses(t, corePkg, "TestRunCycle_ASecondEnforceCloseoutChangesNothingInTheCarryoverLog")
}

func TestC2555_007_AfterOneBoundaryAtEnforceNoPublishedJudgmentTodoRemainsInState(t *testing.T) {
	goTestPasses(t, corePkg, "TestRunCycle_AtEnforceTheJudgmentTodoLeavesOnlyAfterTheBoundaryPublishedItsDocument")
}

func TestC2555_008_ATodoThatIsNoJudgmentLessonIsNeverAskedAboutAndNeverLeaves(t *testing.T) {
	goTestPasses(t, carryoverPkg, "TestJudgmentRetire_ATodoThatIsNotAJudgmentIsNeverAskedAboutAndNeverLeaves")
}

func TestC2555_009_BelowEnforceTheCloseoutRetiresNoJudgmentTodo(t *testing.T) {
	goTestPasses(t, corePkg, "TestRunCycle_BelowEnforceAJudgmentTodoWithItsDocumentOnMainStays")
}
