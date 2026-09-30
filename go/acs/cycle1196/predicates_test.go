//go:build acs

package cycle1196

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const corePkg = "github.com/mickeyyaya/evolve-loop/go/internal/core"

func runGoTest(t *testing.T, pkg, pattern string) (ok bool, out string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-run", "^("+pattern+")$", "-count=1", pkg)
	out = stdout + stderr
	if code < 0 {
		t.Fatalf("go test failed to launch for %s (%s): code=%d err=%v\n%s", pkg, pattern, code, err, out)
	}
	return code == 0, out
}

func TestC1196_001_LaneBasesOnFetchedOriginTip(t *testing.T) {
	ok, out := runGoTest(t, corePkg, "TestGitWorktree_Create_FetchesOriginBeforeBasingLane")
	if !ok {
		t.Errorf("lane worktrees still fork from the stale local HEAD: Create does not fetch origin before `git worktree add`, or still passes \"HEAD\" as the start-ref\n%s", out)
	}
}

func TestC1196_002_NoOriginFallsBackToLocalHEAD(t *testing.T) {
	ok, out := runGoTest(t, corePkg, "TestGitWorktree_Create_NoOriginFallsBackToLocalHEAD")
	if !ok {
		t.Errorf("Create no longer provisions cleanly in a repo with no origin remote (must skip the fetch and base on local HEAD)\n%s", out)
	}
}

func TestC1196_003_FetchFailureIsFatal(t *testing.T) {
	ok, out := runGoTest(t, corePkg, "TestGitWorktree_Create_FetchFailureIsFatal")
	if !ok {
		t.Errorf("a failed origin fetch does not fail loudly: Create still provisions the lane from the stale local tip instead of returning a wrapped fetch error\n%s", out)
	}
}

func TestC1196_004_ExistingProvisioningContractsIntact(t *testing.T) {
	ok, out := runGoTest(t, corePkg,
		"TestGitWorktree_Create_ReuseSkipsFetch|TestGitWorktree_RelativeBaseRefused|TestGitWorktree_RelativeProjectRootRefused|TestOrchestrator_ProvisionsWorktree_PassesToSourcePhases")
	if !ok {
		t.Errorf("the fetch-before-base change regressed existing worktree provisioning contracts (reuse no-op / absolute-base guards / orchestrator provisioning)\n%s", out)
	}
}
