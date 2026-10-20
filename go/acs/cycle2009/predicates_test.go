//go:build acs

package cycle2009

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	cmdPkg      = "github.com/mickeyyaya/evolve-loop/go/cmd/evolve"
	acssuitePkg = "github.com/mickeyyaya/evolve-loop/go/internal/acssuite"
	flagsTests  = "TestFlagsGenerateThenCheck_RoundTrip|TestFlagsCheck_DriftExitsTwo|TestFlagsCheck_ResolvesWorktreeRootOverProjectRoot|TestFlagsUnknownSubcommand"
)

func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

func TestC2009_001_FlagsTestsPassWhenLaneWorktreeRootIsSet(t *testing.T) {
	elsewhere := t.TempDir()
	t.Setenv("EVOLVE_WORKTREE_ROOT", elsewhere)
	ok, out := runGoTest(t, cmdPkg, flagsTests)
	if !ok {
		t.Errorf("TestFlags* still resolve the flags doc through EVOLVE_WORKTREE_ROOT=%s instead of their temp project:\n%s", elsewhere, out)
	}
}

func TestC2009_002_FlagsGenerateNeverWritesOutsideItsTempRoot(t *testing.T) {
	elsewhere := t.TempDir()
	docDir := filepath.Join(elsewhere, "docs", "architecture")
	if err := os.MkdirAll(docDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sentinel := []byte("# Control Flags Reference\n\nuntouched sentinel prose\n")
	docPath := filepath.Join(docDir, "control-flags.md")
	if err := os.WriteFile(docPath, sentinel, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVOLVE_WORKTREE_ROOT", elsewhere)

	runGoTest(t, cmdPkg, flagsTests)

	got, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(sentinel) {
		t.Errorf("flags generate wrote into the foreign EVOLVE_WORKTREE_ROOT doc; it now reads:\n%s", got)
	}
}

func TestC2009_003_PredicateEnvTestStaysGreenUnderLaneRoots(t *testing.T) {
	t.Setenv("EVOLVE_WORKTREE_ROOT", t.TempDir())
	t.Setenv("EVOLVE_PROJECT_ROOT", t.TempDir())
	ok, out := runGoTest(t, acssuitePkg, "TestPredicateEnv_AllBranches")
	if !ok {
		t.Errorf("TestPredicateEnv_AllBranches regressed under a lane's roots:\n%s", out)
	}
}
