//go:build acs

package cycle1150

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	phasecmdPkg = "github.com/mickeyyaya/evolve-loop/go/internal/cli/phasecmd"
	corePkg     = "github.com/mickeyyaya/evolve-loop/go/internal/core"
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

func TestC1150_001_ArchClassBuildWithoutDocsFailsTheSelfCheck(t *testing.T) {
	ok, out := runGoTest(t, phasecmdPkg,
		"TestPhaseVerify_ArchitectureClassDiffWithoutDocs_Exit1|TestPhaseVerify_ArchitectureClassDiffWithoutDocs_JSONCarriesCode")
	if !ok {
		t.Errorf("`evolve phase verify build` does not reject an undocumented architecture-class diff:\n%s", out)
	}
}

func TestC1150_002_DocsDeltaPassesTheSelfCheck(t *testing.T) {
	ok, out := runGoTest(t, phasecmdPkg, "TestPhaseVerify_ArchitectureClassDiffWithDocs_Exit0")
	if !ok {
		t.Errorf("a documented architecture-class build must still pass the self-check:\n%s", out)
	}
}

func TestC1150_003_OrdinaryCyclesAreUnaffected(t *testing.T) {
	ok, out := runGoTest(t, phasecmdPkg,
		"TestPhaseVerify_NonArchitectureDiffInWorktree_Exit0|TestPhaseVerify_NoWorktree_ByteIdentical|TestPhaseVerify_NonBuildPhase_UnaffectedByDocsFloor")
	if !ok {
		t.Errorf("the wiring must be fail-open for non-architecture diffs, absent worktrees and non-build phases:\n%s", out)
	}
}

func TestC1150_004_ChangedPathsAreOneSourceWithAProjection(t *testing.T) {
	ok, out := runGoTest(t, corePkg, "TestChangedWorktreePaths_ExportedForCLIConsumers|TestChangedWorktreePaths_EmptyWorktreeIsEmpty")
	if !ok {
		t.Errorf("core.ChangedWorktreePaths is not exported with the tracked+untracked semantics the CLI needs:\n%s", out)
	}

	verifySrc := filepath.Join(acsassert.RepoRoot(t), "go/internal/cli/phasecmd/phase_verify.go")
	src, err := os.ReadFile(verifySrc)
	if err != nil {
		t.Fatalf("cannot read %s: %v", verifySrc, err)
	}
	for _, needle := range []string{"VerifyBuildWithChangedPaths", "ChangedWorktreePaths"} {
		if !strings.Contains(string(src), needle) {
			t.Errorf("phase_verify.go must consume %s — a classifier nobody calls is the exact defect this cycle closes", needle)
		}
	}
}

func TestC1150_005_TouchedPackagesStayGreen(t *testing.T) {
	for _, pkg := range []string{
		phasecmdPkg,
		corePkg,
		"github.com/mickeyyaya/evolve-loop/go/internal/deliverable",
	} {
		stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-count=1", pkg)
		out := stdout + stderr
		if code < 0 {
			t.Fatalf("go test failed to launch for %s: code=%d err=%v\n%s", pkg, code, err, out)
		}
		if code != 0 {
			t.Errorf("%s must stay green after the wiring:\n%s", pkg, out)
		}
	}
}
