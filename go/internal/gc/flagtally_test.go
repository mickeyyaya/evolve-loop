package gc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

var (
	flagBranchItem = WorktreeItem{Branch: "cycle-flaga", Action: WorktreeActionFlagUnmerged, Reason: "unmerged orphan branch"}
	flagTreeItem   = WorktreeItem{Path: "/hub/.evolve/worktrees/Cycle_7", Action: WorktreeActionFlagDirty, Reason: "dirty"}
	removeItem     = WorktreeItem{Branch: "cycle-done", Action: WorktreeActionRemove}
	fixedNow       = func() time.Time { return time.Date(2026, 11, 11, 9, 0, 0, 0, time.UTC) }
)

func TestIsFlag(t *testing.T) {
	cases := map[WorktreeAction]bool{
		WorktreeActionFlagDirty:     true,
		WorktreeActionFlagUnmerged:  true,
		WorktreeActionRemove:        false,
		WorktreeActionDeleteBranch:  false,
		WorktreeActionSalvageRemove: false,
	}
	for action, want := range cases {
		if got := IsFlag(WorktreeItem{Action: action}); got != want {
			t.Errorf("IsFlag(%s) = %v, want %v", action, got, want)
		}
	}
}

func TestFlagKey_SeparatesActionBranchAndPath(t *testing.T) {
	a := FlagKey(flagBranchItem)
	if a != "flag-unmerged|cycle-flaga|" {
		t.Errorf("key = %q", a)
	}
	if FlagKey(WorktreeItem{Branch: "cycle-flaga", Action: WorktreeActionFlagDirty}) == a {
		t.Error("two actions on one branch share a key")
	}
}

func TestCountFlags_CountsEachRunAndDropsResolvedFlags(t *testing.T) {
	dir := t.TempDir()
	items := []WorktreeItem{flagBranchItem, removeItem}
	for want := 1; want <= 3; want++ {
		counts, err := CountFlags(dir, items)
		if err != nil {
			t.Fatalf("run %d: %v", want, err)
		}
		if counts[FlagKey(flagBranchItem)] != want || len(counts) != 1 {
			t.Fatalf("run %d: counts = %v, want only the flag at %d", want, counts, want)
		}
	}
	counts, err := CountFlags(dir, []WorktreeItem{flagTreeItem})
	if err != nil {
		t.Fatal(err)
	}
	if counts[FlagKey(flagTreeItem)] != 1 || len(counts) != 1 {
		t.Errorf("a new flag must start at 1 and the resolved flag must leave: %v", counts)
	}
	body, err := os.ReadFile(filepath.Join(dir, FlagTallyFile))
	if err != nil || strings.Contains(string(body), "cycle-flaga") {
		t.Errorf("tally file kept the resolved flag: %q err=%v", body, err)
	}
}

func TestCountFlags_Errors(t *testing.T) {
	t.Run("unreadable tally", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, FlagTallyFile), 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := CountFlags(dir, nil); err == nil || !strings.Contains(err.Error(), "flag tally read") {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("corrupt tally", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, FlagTallyFile), []byte("[1]"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := CountFlags(dir, nil); err == nil || !strings.Contains(err.Error(), "not a count map") {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("read-only dir", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, FlagTallyFile)
		if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(dir, 0o555); err != nil {
			t.Fatal(err)
		}
		defer os.Chmod(dir, 0o755)
		if os.Geteuid() == 0 {
			t.Skip("root ignores directory modes")
		}
		if _, err := CountFlags(dir, nil); err == nil || !strings.Contains(err.Error(), "flag tally write") {
			t.Errorf("err = %v", err)
		}
	})
}

func inboxFiles(t *testing.T, dir string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestFileRepeatedFlags_FilesOnceAtTheThreshold(t *testing.T) {
	inbox := filepath.Join(t.TempDir(), "inbox")
	items := []WorktreeItem{flagBranchItem, flagTreeItem, removeItem}
	one := map[string]int{FlagKey(flagBranchItem): 1, FlagKey(flagTreeItem): 1}
	filed, err := FileRepeatedFlags(inbox, items, one, fixedNow)
	if err != nil || len(filed) != 0 {
		t.Fatalf("a flag seen once filed %v, err=%v", filed, err)
	}
	two := map[string]int{FlagKey(flagBranchItem): 2, FlagKey(flagTreeItem): 1}
	filed, err = FileRepeatedFlags(inbox, items, two, fixedNow)
	if err != nil || len(filed) != 1 {
		t.Fatalf("filed %v, err=%v, want exactly one item", filed, err)
	}
	body, err := os.ReadFile(filed[0])
	if err != nil || !strings.Contains(string(body), "cycle-flaga") {
		t.Fatalf("filed item does not name the branch: %q err=%v", body, err)
	}
	for _, count := range []int{3, 4} {
		two[FlagKey(flagBranchItem)] = count
		filed, err = FileRepeatedFlags(inbox, items, two, fixedNow)
		if err != nil || len(filed) != 0 {
			t.Fatalf("count %d filed %v again, err=%v", count, filed, err)
		}
	}
	if n := len(inboxFiles(t, inbox)); n != 1 {
		t.Errorf("inbox holds %d items, want 1", n)
	}
}

func TestFileRepeatedFlags_ConsumedItemIsNotRefiled(t *testing.T) {
	inbox := filepath.Join(t.TempDir(), "inbox")
	counts := map[string]int{FlagKey(flagBranchItem): 2}
	filed, err := FileRepeatedFlags(inbox, []WorktreeItem{flagBranchItem}, counts, fixedNow)
	if err != nil || len(filed) != 1 {
		t.Fatalf("filed %v err=%v", filed, err)
	}
	consumed := filepath.Join(inbox, inboxbatch.ConsumedDirName)
	if err := os.MkdirAll(consumed, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filed[0], filepath.Join(consumed, filepath.Base(filed[0]))); err != nil {
		t.Fatal(err)
	}
	counts[FlagKey(flagBranchItem)] = FlagRepeatThreshold + 1
	filed, err = FileRepeatedFlags(inbox, []WorktreeItem{flagBranchItem}, counts, fixedNow)
	if err != nil || len(filed) != 0 {
		t.Errorf("a consumed flag item was filed again: %v err=%v", filed, err)
	}
}

func TestFileRepeatedFlags_FilesPathOnlyFlag(t *testing.T) {
	inbox := filepath.Join(t.TempDir(), "inbox")
	counts := map[string]int{FlagKey(flagTreeItem): 2}
	filed, err := FileRepeatedFlags(inbox, []WorktreeItem{flagTreeItem}, counts, fixedNow)
	if err != nil || len(filed) != 1 {
		t.Fatalf("filed %v err=%v", filed, err)
	}
	name := filepath.Base(filed[0])
	if !strings.Contains(name, "gc-flag-dirty-cycle-7-") {
		t.Errorf("item id slug for a path flag: %s", name)
	}
	body, _ := os.ReadFile(filed[0])
	if !strings.Contains(string(body), flagTreeItem.Path) {
		t.Errorf("item does not name the tree path: %s", body)
	}
}

func TestFileRepeatedFlags_ReportsFilingError(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{FlagKey(flagBranchItem): 2}
	filed, err := FileRepeatedFlags(filepath.Join(blocker, "inbox"), []WorktreeItem{flagBranchItem}, counts, fixedNow)
	if err == nil || !strings.Contains(err.Error(), "file flag") || len(filed) != 0 {
		t.Errorf("filed=%v err=%v", filed, err)
	}
}

func TestFileRepeatedFlags_FiledFlagItemsAreConsoleRoutedAndNeverLaneAdmissible(t *testing.T) {
	inbox := filepath.Join(t.TempDir(), "inbox")
	items := []WorktreeItem{flagBranchItem, flagTreeItem}
	counts := map[string]int{FlagKey(flagBranchItem): FlagRepeatThreshold, FlagKey(flagTreeItem): FlagRepeatThreshold}
	filed, err := FileRepeatedFlags(inbox, items, counts, fixedNow)
	if err != nil || len(filed) != 2 {
		t.Fatalf("filed %v, err=%v, want two items", filed, err)
	}
	for _, path := range filed {
		it, _, err := inboxbatch.LoadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		lane, console, reasons := inboxbatch.PartitionConsole([]inboxbatch.Item{it}, nil)
		if len(lane) != 0 || len(console) != 1 {
			t.Fatalf("%s is lane-admissible: lane=%v console=%v", it.ID, lane, console)
		}
		if !strings.Contains(reasons[0], "kind:ref-state (git ref state only the console can change)") {
			t.Errorf("reason = %q", reasons[0])
		}
	}
}

func TestPlanWorktrees_AnOrphanBranchWhoseNetContentHeadHoldsIsDeletedNotFlagged(t *testing.T) {
	r := newSquashRepo(t)
	r.Git("switch", "-q", "-c", "cycle-landed-1")
	commitRepoFile(t, r, "a.txt", "a\n")
	squashMerge(r, "cycle-landed-1")
	commitRepoFile(t, r, "later.txt", "later\n")
	r.Git("switch", "-q", "-c", "cycle-open-2", "HEAD~1")
	commitRepoFile(t, r, "unlanded.txt", "x\n")
	r.Git("switch", "-q", "main")

	opts := WorktreeOptions{
		ProjectRoot:  r.Dir,
		WorktreeBase: filepath.Join(r.Dir, ".evolve", "worktrees"),
		EvolveDir:    filepath.Join(r.Dir, ".evolve"),
		Now:          func() time.Time { return time.Now() },
		Exec:         sysexec.DefaultRunner,
		LeaseTTL:     10 * time.Minute,
	}
	m, err := PlanWorktrees(opts)
	if err != nil {
		t.Fatal(err)
	}
	actions := map[string]WorktreeAction{}
	for _, it := range m.Items {
		actions[it.Branch] = it.Action
	}
	if actions["cycle-landed-1"] != WorktreeActionDeleteBranch {
		t.Errorf("landed branch action = %q, want delete-branch (items %+v)", actions["cycle-landed-1"], m.Items)
	}
	if actions["cycle-open-2"] != WorktreeActionFlagUnmerged {
		t.Errorf("unlanded branch action = %q, want flag-unmerged", actions["cycle-open-2"])
	}
}
