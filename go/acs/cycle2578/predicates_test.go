//go:build acs

package cycle2578

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	integrationPkg = "./internal/integration"
	cmdPkg         = "./cmd/evolve"
)

func runTests(t *testing.T, pkg, pattern string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-count=1", "-v", "-run", pattern, pkg)
	cmd.Dir = acsassert.GoModuleDir(t)
	cmd.WaitDelay = 10 * time.Second
	out, _ := cmd.CombinedOutput()
	return string(out)
}

func requirePass(t *testing.T, out string, names ...string) {
	t.Helper()
	for _, name := range names {
		if !strings.Contains(out, "--- PASS: "+name+" (") {
			t.Errorf("%s did not report PASS; output tail:\n%s", name, acsassert.OutputTail(out, 25))
		}
	}
	if strings.Contains(out, "--- FAIL") {
		t.Errorf("a test reported FAIL; output tail:\n%s", acsassert.OutputTail(out, 25))
	}
}

func TestC2578_001_ARedHeadIsEjectedWithCauseHeadCIAndNoBatchCIStarts(t *testing.T) {
	out := runTests(t, integrationPkg, "^TestTrainPlan_(EjectsPRWhoseOwnCIIsRed|EjectsARedHeadBeforeAnyBatchCIStarts)$")
	requirePass(t, out, "TestTrainPlan_EjectsPRWhoseOwnCIIsRed", "TestTrainPlan_EjectsARedHeadBeforeAnyBatchCIStarts")
}

func TestC2578_002_AHeadWithNoRunOrACancelledRunIsEjected(t *testing.T) {
	out := runTests(t, integrationPkg, "^TestTrainPlan_EjectsAHeadWithNoRunOrACancelledRun$")
	requirePass(t, out, "TestTrainPlan_EjectsAHeadWithNoRunOrACancelledRun")
}

func TestC2578_003_AnUnreadableHeadCIFailsThePlanInsteadOfEjecting(t *testing.T) {
	out := runTests(t, integrationPkg, "^TestTrainPlan_FailsLoudlyWhenTheHeadCICannotBeRead$")
	requirePass(t, out, "TestTrainPlan_FailsLoudlyWhenTheHeadCICannotBeRead")
}

func TestC2578_004_APendingHeadStaysOutOfTheBatchWithoutAnEjection(t *testing.T) {
	out := runTests(t, integrationPkg, "^TestTrainPlan_KeepsAPendingHeadOutWithoutEjecting$")
	requirePass(t, out, "TestTrainPlan_KeepsAPendingHeadOutWithoutEjecting")
}

func TestC2578_005_ABatchOfGreenHeadsPlansAsBefore(t *testing.T) {
	out := runTests(t, integrationPkg, "^TestTrainPlan_ABatchOfGreenHeadsPlansAsBefore$")
	requirePass(t, out, "TestTrainPlan_ABatchOfGreenHeadsPlansAsBefore")
}

func TestC2578_006_ExistingTrainTestsPassUnchanged(t *testing.T) {
	root := acsassert.RepoRoot(t)
	_, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "--quiet", "refs/remotes/origin/main", "--", "go/internal/integration/train_test.go")
	if err != nil || code != 0 {
		t.Fatalf("train_test.go differs from origin/main (code %d, err %v): %s", code, err, stderr)
	}
	out := runTests(t, integrationPkg, "^TestTrain")
	if strings.Contains(out, "--- FAIL") || !strings.Contains(out, "\nok  \t") {
		t.Errorf("the existing train tests did not all pass; output tail:\n%s", acsassert.OutputTail(out, 25))
	}
}

func TestC2578_007_TheBatchCommandReachesTheHeadCIFromItsProductionTrain(t *testing.T) {
	out := runTests(t, cmdPkg, "^TestRunBatch_(EjectsAPRWhoseHeadCIIsRedBeforeTheBatchRuns|DryRunNamesThePRWhoseHeadCIIsRed)$")
	requirePass(t, out, "TestRunBatch_EjectsAPRWhoseHeadCIIsRedBeforeTheBatchRuns", "TestRunBatch_DryRunNamesThePRWhoseHeadCIIsRed")
}
