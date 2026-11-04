//go:build acs

package cycle2129

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	treefencePkg = "./internal/treefence"
	runnerPkg    = "./internal/phases/runner"
)

func requirePass(t *testing.T, pkg string, names ...string) {
	t.Helper()
	moduleDir, err := acsassert.ModuleDir(filepath.Join(acsassert.RepoRoot(t), "go"))
	if err != nil {
		t.Fatalf("RED: no Go module: %v", err)
	}
	pattern := "^(" + strings.Join(names, "|") + ")$"
	stdout, stderr, code, runErr := acsassert.SubprocessOutput("go", "-C", moduleDir, "test", "-count=1", "-timeout", "300s", "-v", "-run", pattern, pkg)
	combined := stdout + stderr
	if runErr != nil || code != 0 {
		t.Fatalf("RED: go test %s -run %s exit=%d err=%v\n%s", pkg, pattern, code, runErr, acsassert.OutputTail(combined, 25))
	}
	for _, name := range names {
		if !strings.Contains(combined, "--- PASS: "+name+" ") {
			t.Errorf("RED: %s did not run and pass in %s\n%s", name, pkg, acsassert.OutputTail(combined, 25))
		}
	}
}

func TestC2129_001_FencedWriteLeavesPatchThatGivesTheWriteBack(t *testing.T) {
	requirePass(t, runnerPkg, "TestRun_FencedWriteLeavesAPatchInTheWorkspaceThatGivesItBack")
	requirePass(t, treefencePkg,
		"TestEnd_FencedWritesLeaveAPatchThatGivesThemBack",
		"TestEnd_PatchHoldsOnlyTheRevertedPaths",
		"TestEnd_UnwritablePatchLocationStillRestoresAndSaysSo")
}

func TestC2129_002_FenceDiagnosticNamesThePatchPath(t *testing.T) {
	requirePass(t, runnerPkg, "TestRun_FenceDiagnosticNamesThePatchPath")
	requirePass(t, treefencePkg, "TestOutcomeDiagnostics_NamesThePatchPath")
}

func TestC2129_003_NoFencedWriteWritesNoPatch(t *testing.T) {
	requirePass(t, runnerPkg, "TestRun_NoFencedWriteWritesNoPatch")
	requirePass(t, treefencePkg, "TestEnd_NoFencedWriteWritesNoPatch")
}
