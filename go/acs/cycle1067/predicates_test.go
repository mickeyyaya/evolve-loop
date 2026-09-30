//go:build acs

package cycle1067

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const shipPkg = "./internal/phases/ship/"

func goDir(t *testing.T) string { return filepath.Join(acsassert.RepoRoot(t), "go") }

func runGoTest(t *testing.T, name string, extraArgs ...string) {
	t.Helper()
	args := append([]string{"test", "-C", goDir(t), "-count=1", "-v"}, extraArgs...)
	args = append(args, "-run", "^"+name+"$", shipPkg)
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", args...)
	out := stdout + stderr
	if err != nil {
		t.Fatalf("go test failed to launch (not a test failure): %v\n%s", err, out)
	}
	if code != 0 {
		t.Fatalf("%s -run %s exited %d\n%s", shipPkg, name, code, out)
	}
	if !strings.Contains(out, "--- PASS: "+name) {
		t.Fatalf("no PASS line for %s (renamed, skipped, or never ran?)\n%s", name, out)
	}
}

func TestC1067_001_CycleShipStagesDeclaredPathsNotAddAll(t *testing.T) {
	runGoTest(t, "TestShipDirect_CycleClass_StagesDeclaredPathsNotAddAll")
}

func TestC1067_002_AReportlessWorkspaceAdoptsNoPathAndNoWorkspaceStagesTheChangedSet(t *testing.T) {
	runGoTest(t, "TestShipDirect_AReportlessWorkspaceAdoptsNoPath")
	runGoTest(t, "TestShipDirect_NoWorkspacePath_StillStagesExplicitly")
}

func TestC1067_003_NoNonReleaseClassEmitsAddAll(t *testing.T) {
	runGoTest(t, "TestShipDirect_NonReleaseClasses_NeverAddAll")
	runGoTest(t, "TestShipDirect_CycleClass_KeepsChurnDiscard")

	stale := "TestShipDirect_CycleClass_KeepsChurnDiscardAndAddAll"
	stdout, stderr, _, err := acsassert.SubprocessOutput(
		"go", "test", "-C", goDir(t), "-count=1", "-v", "-run", "^"+stale+"$", shipPkg)
	if err != nil {
		t.Fatalf("go test failed to launch: %v\n%s", err, stdout+stderr)
	}
	if strings.Contains(stdout+stderr, "=== RUN   "+stale) {
		t.Errorf("stale test %s still exists — it asserts `git add -A` staging that this cycle removed", stale)
	}
}

func TestC1067_004_WorktreeShipCommitExcludesUndeclaredStray(t *testing.T) {
	runGoTest(t, "TestShipFromWorktree_StagesDeclaredPathsOnly_ExcludesUndeclaredStray",
		"-tags", "integration")
}

func TestC1067_005_ShipPackageSuiteGreen(t *testing.T) {
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", goDir(t), "-count=1", shipPkg)
	out := stdout + stderr
	if err != nil {
		t.Fatalf("go test failed to launch: %v\n%s", err, out)
	}
	if code != 0 {
		t.Fatalf("ship package suite is RED (exit %d)\n%s", code, out)
	}
}
