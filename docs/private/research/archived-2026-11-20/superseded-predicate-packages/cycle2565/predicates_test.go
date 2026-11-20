//go:build acs

package cycle2565

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func runNamedTest(t *testing.T, pkg, name string) {
	t.Helper()
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	out, errOut, code, err := acsassert.SubprocessOutput("go", "-C", goDir, "test", "-count=1", "-v", "-run", "^"+name+"$", pkg)
	if err != nil {
		t.Fatalf("go test %s: %v", pkg, err)
	}
	if code != 0 {
		t.Errorf("go test -run %s %s exit=%d\n%s%s", name, pkg, code, out, errOut)
		return
	}
	if !strings.Contains(out, "--- PASS: "+name+" ") {
		t.Errorf("%s did not run and pass:\n%s", name, out)
	}
}

func TestC2565_001_ARedHeadIsEjectedWithCauseHeadCIAndStartsNoBatchCI(t *testing.T) {
	runNamedTest(t, "./internal/integration", "TestTrainPlan_EjectsPRWhoseOwnCIIsRed")
	runNamedTest(t, "./internal/integration", "TestTrainPlan_EjectsARedHeadBeforeAnyBatchCIStarts")
}

func TestC2565_002_ARedHeadWithNoRunOrACancelledRunIsEjectedAndAGhFailureIsLoud(t *testing.T) {
	runNamedTest(t, "./internal/integration", "TestTrainPlan_EjectsAHeadWithNoRunOrACancelledRun")
	runNamedTest(t, "./internal/integration", "TestTrainPlan_FailsLoudlyWhenTheHeadCICannotBeRead")
}

func TestC2565_003_APendingHeadIsKeptOutOfTheBatchWithoutEjection(t *testing.T) {
	runNamedTest(t, "./internal/integration", "TestTrainPlan_KeepsAPendingHeadOutWithoutEjecting")
}

func TestC2565_004_GreenHeadsPlanExactlyAsBefore(t *testing.T) {
	runNamedTest(t, "./internal/integration", "TestTrainPlan_ABatchOfGreenHeadsPlansAsBefore")
}

func TestC2565_005_TheBatchCommandReachesTheHeadCICheck(t *testing.T) {
	runNamedTest(t, "./cmd/evolve", "TestRunBatch_EjectsAPRWhoseHeadCIIsRedBeforeTheBatchRuns")
	runNamedTest(t, "./cmd/evolve", "TestRunBatch_DryRunNamesThePRWhoseHeadCIIsRed")
}
