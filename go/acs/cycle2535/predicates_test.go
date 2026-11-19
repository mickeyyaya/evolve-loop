//go:build acs

package cycle2535

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const entryPointTest = "TestGC_ABranchFlaggedWithAndThenWithoutItsWorktreeMintsOneInboxItem"

const branch = "cycle-keyed-1"

const worktreePath = "/hub/.evolve/worktrees/cycle-keyed-1"

var fixedNow = func() time.Time { return time.Date(2026, 11, 19, 9, 0, 0, 0, time.UTC) }

func withWorktree(action gc.WorktreeAction) gc.WorktreeItem {
	return gc.WorktreeItem{Branch: branch, Path: worktreePath, Action: action, Reason: "unmerged"}
}

func withoutWorktree(action gc.WorktreeAction) gc.WorktreeItem {
	return gc.WorktreeItem{Branch: branch, Action: action, Reason: "unmerged orphan branch (no worktree)"}
}

func pathOnly(path string) gc.WorktreeItem {
	return gc.WorktreeItem{Path: path, Action: gc.WorktreeActionFlagDirty, Reason: "dirty"}
}

func seedTally(t *testing.T, evolveDir string, tally map[string]int) {
	t.Helper()
	body, err := json.Marshal(tally)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, gc.FlagTallyFile), body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func run(t *testing.T, evolveDir string, items ...gc.WorktreeItem) []string {
	t.Helper()
	counts, err := gc.CountFlags(evolveDir, items)
	if err != nil {
		t.Fatalf("CountFlags: %v", err)
	}
	filed, err := gc.FileRepeatedFlags(filepath.Join(evolveDir, "inbox"), items, counts, fixedNow)
	if err != nil {
		t.Fatalf("FileRepeatedFlags: %v", err)
	}
	return filed
}

func inboxItems(t *testing.T, evolveDir string) int {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(evolveDir, "inbox", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	return len(files)
}

func TestC2535_001_TheGCEntryPointMintsOneItemWhenTheBranchLosesItsWorktree(t *testing.T) {
	moduleDir := acsassert.GoModuleDir(t)
	out, errOut, code, err := acsassert.SubprocessOutput("go", "-C", moduleDir, "test", "-count=1", "-v", "-run", "^"+entryPointTest+"$", "./cmd/evolve")
	if err != nil && code < 0 {
		t.Fatalf("go test could not run: %v", err)
	}
	tail := acsassert.OutputTail(out+errOut, 20)
	if code != 0 {
		t.Fatalf("RED: evolve gc minted a second inbox item for one branch (exit %d):\n%s", code, tail)
	}
	if !strings.Contains(out, "--- PASS: "+entryPointTest) {
		t.Errorf("the entry-point test did not run and pass:\n%s", tail)
	}
}

func TestC2535_002_FourRunsOfOneBranchWithThenWithoutWorktreeMintOneItem(t *testing.T) {
	evolveDir := t.TempDir()
	for range 2 {
		run(t, evolveDir, withWorktree(gc.WorktreeActionFlagUnmerged))
	}
	for range 2 {
		run(t, evolveDir, withoutWorktree(gc.WorktreeActionFlagUnmerged))
	}
	if n := inboxItems(t, evolveDir); n != 1 {
		t.Errorf("RED: one branch minted %d inbox items, want 1", n)
	}
}

func TestC2535_003_ABranchWithoutItsWorktreeKeepsTheCountItEarnedWithIt(t *testing.T) {
	evolveDir := t.TempDir()
	run(t, evolveDir, withWorktree(gc.WorktreeActionFlagUnmerged))
	counts, err := gc.CountFlags(evolveDir, []gc.WorktreeItem{withoutWorktree(gc.WorktreeActionFlagUnmerged)})
	if err != nil {
		t.Fatal(err)
	}
	got := counts[gc.FlagKey(withoutWorktree(gc.WorktreeActionFlagUnmerged))]
	if got != 2 {
		t.Errorf("RED: the second run of the branch counts %d, want 2 whether or not the worktree remains", got)
	}
}

func TestC2535_004_AnOldShapeTallyKeepsItsCountForABranchWithAWorktree(t *testing.T) {
	evolveDir := t.TempDir()
	seedTally(t, evolveDir, map[string]int{"flag-unmerged|" + branch + "|" + worktreePath: 1})
	filed := run(t, evolveDir, withWorktree(gc.WorktreeActionFlagUnmerged))
	if len(filed) != 1 {
		t.Errorf("an old-shape count of 1 plus this run must reach the threshold and mint one item, minted %v", filed)
	}
}

func TestC2535_005_AnOldShapeTallyKeepsItsCountForABranchWithoutAWorktree(t *testing.T) {
	evolveDir := t.TempDir()
	seedTally(t, evolveDir, map[string]int{"flag-unmerged|" + branch + "|": 1})
	filed := run(t, evolveDir, withoutWorktree(gc.WorktreeActionFlagUnmerged))
	if len(filed) != 1 {
		t.Errorf("an old-shape count of 1 plus this run must reach the threshold and mint one item, minted %v", filed)
	}
}

func TestC2535_006_APathOnlyFlagStillKeysOnItsPath(t *testing.T) {
	evolveDir := t.TempDir()
	first := pathOnly("/hub/.evolve/worktrees/Cycle_A")
	other := pathOnly("/hub/.evolve/worktrees/Cycle_B")
	counts, err := gc.CountFlags(evolveDir, []gc.WorktreeItem{first, other})
	if err != nil {
		t.Fatal(err)
	}
	if counts[gc.FlagKey(first)] != 1 || counts[gc.FlagKey(other)] != 1 || gc.FlagKey(first) == gc.FlagKey(other) {
		t.Fatalf("two path-only flags share a key: %v", counts)
	}
	filed := run(t, evolveDir, first)
	if len(filed) != 1 || inboxItems(t, evolveDir) != 1 {
		t.Errorf("the same path flagged twice must mint one item, minted %v", filed)
	}
}

func TestC2535_007_AnOldShapeTallyKeepsItsCountForAPathOnlyFlag(t *testing.T) {
	evolveDir := t.TempDir()
	seedTally(t, evolveDir, map[string]int{"flag-dirty||" + worktreePath: 1})
	filed := run(t, evolveDir, pathOnly(worktreePath))
	if len(filed) != 1 {
		t.Errorf("an old-shape count of 1 plus this run must reach the threshold and mint one item, minted %v", filed)
	}
}

func TestC2535_008_DifferentBranchesAndDifferentActionsNeverShareACount(t *testing.T) {
	evolveDir := t.TempDir()
	unmerged := withWorktree(gc.WorktreeActionFlagUnmerged)
	dirty := withWorktree(gc.WorktreeActionFlagDirty)
	otherBranch := gc.WorktreeItem{Branch: "cycle-keyed-2", Path: "/hub/.evolve/worktrees/cycle-keyed-2", Action: gc.WorktreeActionFlagUnmerged, Reason: "unmerged"}
	counts, err := gc.CountFlags(evolveDir, []gc.WorktreeItem{unmerged, dirty, otherBranch})
	if err != nil {
		t.Fatal(err)
	}
	keys := map[string]bool{gc.FlagKey(unmerged): true, gc.FlagKey(dirty): true, gc.FlagKey(otherBranch): true}
	if len(keys) != 3 {
		t.Errorf("RED: keys collide across branch or action: %v", counts)
	}
	for key, n := range counts {
		if n != 1 {
			t.Errorf("first run of %q counts %d, want 1", key, n)
		}
	}
}
