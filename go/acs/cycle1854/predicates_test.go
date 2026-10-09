//go:build acs

package cycle1854

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func runNamedTests(t *testing.T, pkg, pattern string) string {
	t.Helper()
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", goDir, "-count=1", "-v", "-run", pattern, pkg)
	out := stdout + "\n" + stderr
	if err != nil {
		t.Fatalf("go test %s: %v", pkg, err)
	}
	if strings.Contains(out, "no tests to run") {
		t.Fatalf("RED: pattern %s matched no test in %s", pattern, pkg)
	}
	if code != 0 {
		t.Errorf("RED: go test -run %s %s exit=%d\n%s", pattern, pkg, code, out)
	}
	return out
}

func requirePassed(t *testing.T, out string, names ...string) {
	t.Helper()
	for _, n := range names {
		if !strings.Contains(out, "--- PASS: "+n+" ") {
			t.Errorf("RED: %s did not run and PASS", n)
		}
	}
}

func TestC1854_001_VersionInventoryProbedOncePerRun(t *testing.T) {
	out := runNamedTests(t, "./internal/looppreflight",
		`^(TestRun_VersionInventoryProbedOncePerRun|TestRun_ReportedVersionsMatchSavedCache|TestRun_DefaultInventoryProbesEachBinaryOnce)$`)
	requirePassed(t, out,
		"TestRun_VersionInventoryProbedOncePerRun",
		"TestRun_ReportedVersionsMatchSavedCache",
		"TestRun_DefaultInventoryProbesEachBinaryOnce")
}

func TestC1854_002_VersionCacheWritesAreAtomicAndCollisionFree(t *testing.T) {
	out := runNamedTests(t, "./internal/looppreflight",
		`^(TestSaveVersionCache_SurvivesStaticTempNameCollision|TestSaveVersionCache_ConcurrentWritersLeaveValidJSON|TestRun_CacheWrittenDespiteStaticTempOccupied)$`)
	requirePassed(t, out,
		"TestSaveVersionCache_SurvivesStaticTempNameCollision",
		"TestSaveVersionCache_ConcurrentWritersLeaveValidJSON",
		"TestRun_CacheWrittenDespiteStaticTempOccupied")
}

func TestC1854_003_PreflightUnitTestsExecNoRealSubprocess(t *testing.T) {
	out := runNamedTests(t, "./internal/looppreflight",
		`^(TestGoodPipelineOptions_StubsEveryProcessSeam|TestRun_GoodPipelineOptionsExecNoVersionProbe)$`)
	requirePassed(t, out,
		"TestGoodPipelineOptions_StubsEveryProcessSeam",
		"TestRun_GoodPipelineOptionsExecNoVersionProbe")
}

func TestC1854_004_Cycle270PredicatesPassAgainstCurrentCode(t *testing.T) {
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", goDir, "-count=1", "-tags", "acs", "./acs/cycle270")
	if err != nil {
		t.Fatalf("go test ./acs/cycle270: %v", err)
	}
	if code != 0 {
		t.Errorf("RED: go test -tags acs ./acs/cycle270 exit=%d (TestC270_004 pins the removed TestDefaultTmuxSessions)\n%s\n%s", code, stdout, stderr)
	}
}
