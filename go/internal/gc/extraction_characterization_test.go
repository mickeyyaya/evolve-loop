package gc

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

func TestPlan_DispatchLogTTLSkipsNonLogFiles(t *testing.T) {
	dir := t.TempDir()
	logs := filepath.Join(dir, "dispatch-logs")
	keep := filepath.Join(logs, "keep.txt")
	old := filepath.Join(logs, "batch-1.log")
	for _, p := range []string{keep, old} {
		writeFile(t, p, "x")
		if err := os.Chtimes(p, daysAgo(100), daysAgo(100)); err != nil {
			t.Fatal(err)
		}
	}
	m, err := Plan(Options{EvolveDir: dir, Now: nowT0})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	items := planItems(t, m)
	if _, ok := items[keep]; ok {
		t.Errorf("a non-.log file in dispatch-logs must never be planned: %+v", m.Items)
	}
	if it := items[old]; it.Action != ActionDelete || it.Rule != "logs.dispatch.ttl_days" {
		t.Errorf("an expired .log file must be deleted by the dispatch catalog TTL (logs.dispatch.ttl_days), got %+v", it)
	}
}

func TestPlan_TrackerTTLIgnoresEphemeralRegularFile(t *testing.T) {
	dir := t.TempDir()
	kept := mkRun(t, dir, "cycle-60", daysAgo(1))
	eph := filepath.Join(kept.Path, ".ephemeral")
	writeFile(t, eph, "not a tracker dir")
	if err := os.Chtimes(eph, daysAgo(30), daysAgo(30)); err != nil {
		t.Fatal(err)
	}
	m, err := Plan(Options{EvolveDir: dir, Runs: []RunDir{kept}, Now: nowT0})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(m.Items) != 0 {
		t.Errorf("a regular .ephemeral file is not a tracker subtree and must not be planned: %+v", m.Items)
	}
}

func TestPlan_DeletedRunIsNotRescannedForTrackerTTL(t *testing.T) {
	dir := t.TempDir()
	fresh := mkRun(t, dir, "cycle-71", daysAgo(1))
	old := mkRun(t, dir, "cycle-70", daysAgo(100))
	eph := filepath.Join(old.Path, ".ephemeral")
	if err := os.MkdirAll(eph, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(eph, daysAgo(100), daysAgo(100)); err != nil {
		t.Fatal(err)
	}
	m, err := Plan(Options{
		EvolveDir: dir,
		Policy:    Policy{Runs: RunsPolicy{KeepFull: 1, DeleteAfterDays: 30}},
		Runs:      []RunDir{fresh, old},
		Now:       nowT0,
	})
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	if len(m.Items) != 1 {
		t.Fatalf("want exactly the old run's delete (its .ephemeral goes with it), got %+v", m.Items)
	}
	want := Item{Path: old.Path, Action: ActionDelete, Rule: "runs.delete_after_days"}
	if m.Items[0] != want {
		t.Errorf("want %+v, got %+v", want, m.Items[0])
	}
}

func TestApply_ArchiveSkipsDanglingSymlinkEntry(t *testing.T) {
	dir := t.TempDir()
	run := mkRun(t, dir, "cycle-94", daysAgo(90))
	archiveDir := filepath.Join(dir, "archive", "runs")
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	dangling := filepath.Join(archiveDir, "cycle-94")
	if err := os.Symlink(filepath.Join(dir, "missing-target"), dangling); err != nil {
		t.Fatal(err)
	}
	if err := Apply(dir, Manifest{Items: []Item{{Path: run.Path, Action: ActionArchive}}}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if info, err := os.Lstat(dangling); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("the dangling archive entry must survive untouched: info=%v err=%v", info, err)
	}
	if info, err := os.Stat(filepath.Join(archiveDir, "cycle-94.1")); err != nil || !info.IsDir() {
		t.Errorf("the run must be archived under the numeric suffix: info=%v err=%v", info, err)
	}
}

func TestApply_ArchiveErrorWrapsItsCause(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "runs", "cycle-404")
	err := Apply(dir, Manifest{Items: []Item{{Path: missing, Action: ActionArchive}}})
	if err == nil {
		t.Fatal("archiving a missing source must fail")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the archive error must wrap the rename cause (fs.ErrNotExist), got %v", err)
	}
}

func TestDiscover_LedgerRefsArePathCleaned(t *testing.T) {
	dir := t.TempDir()
	manual := filepath.Join(dir, "runs", "manual-x")
	if err := os.MkdirAll(manual, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := Discover(dir, DiscoverOptions{Now: nowT0, LedgerRefs: []string{manual + "/"}})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(got) != 1 || got[0].Path != manual {
		t.Errorf("a ledger ref with a trailing slash must still evidence %s, got %+v", manual, got)
	}
}

func TestDiscover_HonorsLeaseTTLOption(t *testing.T) {
	dir := t.TempDir()
	run := filepath.Join(dir, "runs", "cycle-8")
	writeFile(t, filepath.Join(run, "run.json"), `{"cycle_id":8}`)
	if err := runlease.Write(run, runlease.Lease{RunID: "r8"}, t0.Add(-30*time.Minute)); err != nil {
		t.Fatal(err)
	}
	got, err := Discover(dir, DiscoverOptions{Now: nowT0, LeaseTTL: time.Hour})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(got) != 1 || !got[0].Live {
		t.Errorf("a 30m-old lease is fresh under a 1h LeaseTTL, got %+v", got)
	}
}

func TestPlanWorktrees_IgnoresNonCandidates(t *testing.T) {
	cases := []struct {
		name  string
		setup func(e *worktreesTestEnv)
	}{
		{"non-cycle leaf", func(e *worktreesTestEnv) {
			e.addWorktree("feature-x", "feature-x", 20*time.Hour, false, true)
		}},
		{"detached worktree", func(e *worktreesTestEnv) {
			e.addWorktree("cycle-det-5", "", 20*time.Hour, false, true)
		}},
		{"outside WorktreeBase", func(e *worktreesTestEnv) {
			outside := filepath.Join(e.projectRoot, "elsewhere", "cycle-out-9")
			if err := os.MkdirAll(outside, 0o755); err != nil {
				e.t.Fatal(err)
			}
			e.git.porcelain = buildPorcelain([]worktreeFixture{{path: outside, branch: "cycle-out-9"}})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newWorktreesTestEnv(t)
			tc.setup(e)
			m, err := PlanWorktrees(e.opts())
			if err != nil {
				t.Fatalf("PlanWorktrees: %v", err)
			}
			if len(m.Items) != 0 {
				t.Errorf("a non-candidate worktree must produce no items: %+v", m.Items)
			}
		})
	}
}

func TestPlanWorktrees_CheckedOutBranchIsNotAnOrphan(t *testing.T) {
	e := newWorktreesTestEnv(t)
	wt := e.addWorktree("cycle-ccc3333-572", "cycle-ccc3333-572", 20*time.Hour, true, true)
	e.git.backlogBranches = []string{"cycle-ccc3333-572"}
	m, err := PlanWorktrees(e.opts())
	if err != nil {
		t.Fatalf("PlanWorktrees: %v", err)
	}
	if len(m.Items) != 1 || m.Items[0].Action != WorktreeActionFlagDirty || m.Items[0].Path != wt {
		t.Errorf("a checked-out branch must only be flagged via its worktree, never swept as an orphan: %+v", m.Items)
	}
}

func TestPlanWorktrees_ItemsUseCallerRelativePath(t *testing.T) {
	e := newWorktreesTestEnv(t)
	real := filepath.Join(e.projectRoot, "real-worktrees")
	if err := os.MkdirAll(filepath.Join(real, "cycle-sym-6"), 0o755); err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(real)
	if err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(e.projectRoot, "linked-worktrees")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	e.git.porcelain = buildPorcelain([]worktreeFixture{{path: filepath.Join(resolved, "cycle-sym-6"), branch: "cycle-sym-6"}})
	e.git.mergedBranches["cycle-sym-6"] = true
	opts := e.opts()
	opts.WorktreeBase = link
	m, err := PlanWorktrees(opts)
	if err != nil {
		t.Fatalf("PlanWorktrees: %v", err)
	}
	want := filepath.Join(link, "cycle-sym-6")
	removes := itemsWithAction(m.Items, WorktreeActionRemove)
	if len(removes) != 1 || removes[0].Path != want {
		t.Errorf("items must carry WorktreeBase/leaf %s, not git's resolved path: %+v", want, m.Items)
	}
}

func TestApplyWorktrees_RefusedPathIsReportedOnce(t *testing.T) {
	e := newWorktreesTestEnv(t)
	wt := e.addWorktree("cycle-ddd4444-702", "cycle-ddd4444-702", 20*time.Hour, false, true)
	opts := e.opts()
	m, err := PlanWorktrees(opts)
	if err != nil {
		t.Fatalf("PlanWorktrees: %v", err)
	}
	if len(m.Items) != 2 {
		t.Fatalf("setup: want remove + delete-branch for %s, got %+v", wt, m.Items)
	}
	e.git.dirtyDirs[wt] = true
	err = ApplyWorktrees(opts, m)
	if err == nil {
		t.Fatal("a now-dirty target must be refused")
	}
	if n := strings.Count(err.Error(), "became dirty between plan and apply"); n != 1 {
		t.Errorf("a refused path must be reported exactly once, got %d: %v", n, err)
	}
}

func TestApplyWorktrees_DeleteBranchOnlyItemIsRecheckedForLiveness(t *testing.T) {
	e := newWorktreesTestEnv(t)
	wt := e.addWorktree("cycle-eee5555-703", "cycle-eee5555-703", 20*time.Hour, false, true)
	writeFile(t, filepath.Join(e.evolveDir, "cycle-state.json"), `{"active_worktree":"`+wt+`"}`)
	m := WorktreeManifest{Items: []WorktreeItem{{Path: wt, Branch: "cycle-eee5555-703", Action: WorktreeActionDeleteBranch}}}
	err := ApplyWorktrees(e.opts(), m)
	if err == nil || !strings.Contains(err.Error(), "became live between plan and apply") {
		t.Errorf("a now-live target must be refused as live, got %v", err)
	}
	if n := e.git.callCount("branch -d"); n != 0 {
		t.Errorf("a now-live target's branch must never be deleted: calls=%v", e.git.calls)
	}
}
