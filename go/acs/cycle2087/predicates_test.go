//go:build acs

package cycle2087

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func goTestInCmdEvolve(t *testing.T, tags, runPattern string) {
	t.Helper()
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	args := []string{"-C", goDir, "test", "-count=1", "-run", runPattern}
	if tags != "" {
		args = append(args, "-tags", tags)
	}
	args = append(args, "./cmd/evolve")
	out, stderr, code, err := acsassert.SubprocessOutput("go", args...)
	if err != nil || code != 0 {
		t.Errorf("RED: go test -run %s ./cmd/evolve exit=%d err=%v\n%s", runPattern, code, err, acsassert.OutputTail(out+stderr, 25))
	}
}

func TestC2087_001_RebuildVerbBuildsAndPrintsBothSHAs(t *testing.T) {
	goTestInCmdEvolve(t, "", "^TestRunRebuild_SuccessPrintsSHAs$")
}

func TestC2087_002_LiveLeaseExits1AndFailedBuildExits2WithTail(t *testing.T) {
	goTestInCmdEvolve(t, "", "^(TestRunRebuild_LiveRunLeaseExits1|TestRunRebuild_BuildFailureExits2WithTail)$")
}

func TestC2087_003_BootRefreshChainBoundaryAndVerbShareOneBuildFunction(t *testing.T) {
	goTestInCmdEvolve(t, "", "^(TestMakeGoBuild_TheCommandHasExactlyOneCallSite|TestRebuildVerbAndBootRefreshDefaultToOneBuildFunction)$")
}

func TestC2087_004_ChainBuildFailureStillCarriesTheOutputTail(t *testing.T) {
	goTestInCmdEvolve(t, "integration", "^(TestDefaultChainRebuild_SucceedsQuietlyOrCarriesTheBuildOutput|TestDefaultBootRefreshRebuild_RunsTheCanonicalBuildWithChatterOnStderr|TestDefaultBootRefreshRebuild_ABuildFailureIsItsExit)$")
}

func TestC2087_005_TheMakeBuildCommandIsSpawnedFromOneNonTestSite(t *testing.T) {
	root := acsassert.RepoRoot(t)
	out, _, code, err := acsassert.SubprocessOutput("grep", "-rn", "--include=*.go", "--exclude=*_test.go", `"make", "-C", "go", "build"`, filepath.Join(root, "go", "cmd", "evolve"))
	if err != nil && code != 1 {
		t.Fatalf("grep failed: code=%d err=%v", code, err)
	}
	sites := strings.Count(strings.TrimSpace(out), "\n") + 1
	if strings.TrimSpace(out) == "" {
		sites = 0
	}
	if sites != 1 {
		t.Errorf("RED: %d non-test `make -C go build` sites in cmd/evolve, want 1:\n%s", sites, out)
	}
}
