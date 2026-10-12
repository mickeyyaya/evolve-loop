//go:build acs

package cycle1870

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func goTest(t *testing.T, pkg, run string) (string, int) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	out, errOut, code, err := acsassert.SubprocessOutput("go", "-C", filepath.Join(root, "go"), "test", "-count=1", "-run", run, pkg)
	if err != nil && code < 0 {
		t.Fatalf("go test %s: %v", pkg, err)
	}
	return out + errOut, code
}

func TestC1870_001_SharedStateReaderPinsAbsentAndMalformed(t *testing.T) {
	out, code := goTest(t, "./internal/cyclestate", "^(TestReadLastCycleNumber|TestReadState)$")
	if code != 0 {
		t.Errorf("RED: cyclestate shared reader tests do not pass:\n%s", out)
	}
}

func TestC1870_002_WaveCycleFloorUsesUnifiedSemantics(t *testing.T) {
	out, code := goTest(t, "./internal/wave", "^(TestCycleFloor|TestCycleFloor_ExcludesTheUnfinalizedLaneOfThePreviousWave)$")
	if code != 0 {
		t.Errorf("RED: wave.CycleFloor regressed:\n%s", out)
	}
}

func TestC1870_003_RedteamStillRejectsBatchJumpAndHasNoPrivateDecoder(t *testing.T) {
	out, code := goTest(t, "./internal/redteamcheck", "^(TestNoBatchCycleJump|TestLedgerRoleCompleteness|TestChallengeTokenIntegrity)$")
	if code != 0 {
		t.Errorf("RED: redteamcheck regressed:\n%s", out)
	}
	root := acsassert.RepoRoot(t)
	acsassert.FileNotContains(t, filepath.Join(root, "go/internal/redteamcheck/redteamcheck.go"), "func stateLastCycle(")
}

func TestC1870_004_WaveFactsListSquashMergedPRs(t *testing.T) {
	out, code := goTest(t, "./cmd/evolve", "^TestParseMergedPRs")
	if code != 0 {
		t.Errorf("RED: squash-merge subjects are not listed:\n%s", out)
	}
	if !strings.Contains(out, "ok") && code == 0 {
		t.Errorf("RED: no test ran:\n%s", out)
	}
}

func TestC1870_005_FirstParentLogWithoutMergesFlagCountsSquashCommits(t *testing.T) {
	root := acsassert.RepoRoot(t)
	out, errOut, code, err := acsassert.SubprocessOutput("go", "-C", filepath.Join(root, "go"), "test", "-count=1", "-tags", "integration", "-run", "^(TestGitMergedPRs_ListsSquashMergedPRs|TestGitWaveAdapters_ReadTheHeadAndTheMergedPRs)$", "./cmd/evolve")
	if err != nil && code < 0 {
		t.Fatalf("go test: %v", err)
	}
	if code != 0 {
		t.Errorf("RED: real-git squash/merge listing fails:\n%s%s", out, errOut)
	}
}
