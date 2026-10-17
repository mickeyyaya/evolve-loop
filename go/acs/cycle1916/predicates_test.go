//go:build acs

package cycle1916

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func runCoreFloorTests(t *testing.T, names ...string) {
	t.Helper()
	module := filepath.Join(acsassert.RepoRoot(t), "go")
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "-C", module, "test", "-count=1", "-v", "-run", "^("+strings.Join(names, "|")+")$", "./internal/core")
	if err != nil || code != 0 {
		t.Fatalf("go test %v exit=%d err=%v\n%s\n%s", names, code, err, stdout, stderr)
	}
	for _, name := range names {
		if !strings.Contains(stdout, "--- PASS: "+name+" ") {
			t.Errorf("%s did not run and pass", name)
		}
	}
}

func TestC1916_001_TagOnlyACSPackageNeverReachesTheCoverProfiler(t *testing.T) {
	runCoreFloorTests(t, "TestCoverChangedPackages_ExcludesTagOnly", "TestCoverChangedFailures_ExcludesACSTagOnlyPackageFromProfiler", "TestCoverChangedPackages_KeepsOnlyLivePackagesOutsideTestdata")
}

func TestC1916_002_AnAddedUntestedFunctionIsRefusedAtEnforceAndWarnedAtShadow(t *testing.T) {
	runCoreFloorTests(t, "TestCoverChangedFailures_UncoveredFunction")
}

func TestC1916_003_AChangedStrictPackageBelowItsFloorFailsAtEnforceAndWarnsAtShadow(t *testing.T) {
	runCoreFloorTests(t, "TestCoverChangedFailures_StrictRunner", "TestCoverChangedFailures_StrictRunnerDefaultMeasuresOnlyTheChangedStrictPackages")
}

func TestC1916_004_ThePreExistingCoverChangedContractStillHolds(t *testing.T) {
	runCoreFloorTests(t, "TestCoverChangedFailures_EnforceRefusesTheChangedFunctionOfTheBuild", "TestCoverChangedFailures_AFailingTestRefusesAtEnforceAndOnlyWarnsAtShadow", "TestCoverChangedFailures_ShadowOffAndEveryPlumbingFaultLeaveTheHandoffAlone", "TestCoverChangedResult_EnforceRejectsEveryGapByName")
}
