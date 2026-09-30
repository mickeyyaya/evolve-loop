//go:build acs

package cycle667

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const corePkg = "./internal/core/..."

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func runNamedTest(t *testing.T, pkg, name string) (passed bool, out string) {
	t.Helper()
	stdout, stderr, _, _ := acsassert.SubprocessOutput(
		"go", "test", "-C", goDir(t), "-race", "-count=1", "-v",
		"-run", "^"+name+"$", pkg,
	)
	out = stdout + "\n" + stderr
	return strings.Contains(out, "--- PASS: "+name), out
}

func TestC667_001_TerminalHookMergesMemoIntoPersistedState(t *testing.T) {
	passed, out := runNamedTest(t, corePkg, "TestRunCycle_MergesMemoCarryoverTodosIntoState")
	if !passed {
		t.Fatalf("RED: TestRunCycle_MergesMemoCarryoverTodosIntoState did not run+PASS.\n"+
			"Builder must call MergeWorkspaceCarryover(state, cs.WorkspacePath, cycle, now) in\n"+
			"finalizeCycle (beside persistCycleEndState) so a workspace carryover-todos.json\n"+
			"lands in the PERSISTED state — closing the orphaned PASS/retro learning channel.\nOutput:\n%s", out)
	}
}

func TestC667_002_MergeDedupesById(t *testing.T) {
	passed, out := runNamedTest(t, corePkg, "TestMergeWorkspaceCarryover_DedupesById")
	if !passed {
		t.Fatalf("RED: TestMergeWorkspaceCarryover_DedupesById did not run+PASS.\n"+
			"MergeWorkspaceCarryover must dedup by id via the existing mergeCarryoverTodos so a\n"+
			"crash-resume / double-invocation never duplicates a todo.\nOutput:\n%s", out)
	}
}

func TestC667_003_MergeCapsActionRunes(t *testing.T) {
	passed, out := runNamedTest(t, corePkg, "TestMergeWorkspaceCarryover_CapsActionRunes")
	if !passed {
		t.Fatalf("RED: TestMergeWorkspaceCarryover_CapsActionRunes did not run+PASS.\n"+
			"MergeWorkspaceCarryover must cap the decoded action via capRunes(action,\n"+
			"maxAdoptedDefectRunes) so a memo todo cannot bloat every future router prompt.\nOutput:\n%s", out)
	}
}

func TestC667_004_MergeMalformedTolerated(t *testing.T) {
	passed, out := runNamedTest(t, corePkg, "TestMergeWorkspaceCarryover_MalformedFileWarnsNotFails")
	if !passed {
		t.Fatalf("RED: TestMergeWorkspaceCarryover_MalformedFileWarnsNotFails did not run+PASS.\n"+
			"MergeWorkspaceCarryover must tolerant-decode: a malformed file WARNs (never fatals),\n"+
			"and entries missing id or action are skipped. The cycle-terminal hook must never\n"+
			"abort the cycle over a bad memo file.\nOutput:\n%s", out)
	}
}

func TestC667_005_MergeStampsExpiryForPrune(t *testing.T) {
	passed, out := runNamedTest(t, corePkg, "TestMergeWorkspaceCarryover_StampsExpiryForPrune")
	if !passed {
		t.Fatalf("RED: TestMergeWorkspaceCarryover_StampsExpiryForPrune did not run+PASS.\n"+
			"MergeWorkspaceCarryover must stamp FirstSeenCycle + a future ExpiresAt (same TTL\n"+
			"discipline as the loop-start backfill, e.g. failurelog.ComputeExpiresAt) so\n"+
			"failurelog.PruneExpiredCarryoverTodos can age the array out.\nOutput:\n%s", out)
	}
}

func TestC667_006_CoreBuildsAndMergeTestsRaceClean(t *testing.T) {
	dir := goDir(t)
	if _, errOut, code, err := acsassert.SubprocessOutput(
		"go", "build", "-C", dir, "./internal/core/...",
	); code != 0 || err != nil {
		t.Fatalf("RED: internal/core does not build against the MergeWorkspaceCarryover surface (exit=%d): %v\n%s",
			code, err, errOut)
	}
	stdout, stderr, _, _ := acsassert.SubprocessOutput(
		"go", "test", "-C", dir, "-race", "-count=1", "-v",
		"-run", "^TestMergeWorkspaceCarryover_", corePkg,
	)
	out := stdout + "\n" + stderr
	if strings.Contains(out, "DATA RACE") {
		t.Fatalf("RED: DATA RACE detected in MergeWorkspaceCarryover tests:\n%s", out)
	}
	if !strings.Contains(out, "--- PASS: TestMergeWorkspaceCarryover_DedupesById") {
		t.Fatalf("RED: MergeWorkspaceCarryover tests did not pass under -race.\nOutput:\n%s", out)
	}
}
