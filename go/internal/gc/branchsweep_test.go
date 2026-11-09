package gc

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

var branchSweepOld = time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)

type branchSweepEnv struct {
	t    *testing.T
	repo *gittest.Repo
	opts WorktreeOptions
}

func newBranchSweepEnv(t *testing.T) *branchSweepEnv {
	t.Helper()
	r := gittest.Fixture(t)
	commitRepoFile(t, r, "seed.txt", "seed\n")
	root := r.Dir
	return &branchSweepEnv{t: t, repo: r, opts: WorktreeOptions{
		ProjectRoot:  root,
		WorktreeBase: filepath.Join(root, ".evolve", "worktrees"),
		EvolveDir:    filepath.Join(root, ".evolve"),
		Exec:         sysexec.DefaultRunner,
		BranchScope:  BranchScopeAll,
		EndedAfter:   14 * 24 * time.Hour,
	}}
}

func (e *branchSweepEnv) commitOn(branch, file string) string {
	e.t.Helper()
	e.repo.Git("checkout", "-q", "-b", branch)
	commitRepoFile(e.t, e.repo, file, branch+"\n")
	tip := e.repo.Git("rev-parse", "HEAD")
	e.repo.Git("checkout", "-q", "main")
	return tip
}

func (e *branchSweepEnv) land(branch, file string) string {
	e.t.Helper()
	tip := e.commitOn(branch, file)
	e.repo.Git("merge", "-q", "--no-ff", "-m", "land "+branch, branch)
	return tip
}

func (e *branchSweepEnv) end(branch, file string) string {
	e.t.Helper()
	stamp := branchSweepOld.Format(time.RFC3339)
	e.t.Setenv("GIT_AUTHOR_DATE", stamp)
	e.t.Setenv("GIT_COMMITTER_DATE", stamp)
	tip := e.commitOn(branch, file)
	os.Unsetenv("GIT_AUTHOR_DATE")
	os.Unsetenv("GIT_COMMITTER_DATE")
	return tip
}

func (e *branchSweepEnv) tip(branch string) string {
	out, err := exec.Command("git", "-C", e.repo.Dir, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func (e *branchSweepEnv) bundles() []string {
	found, err := filepath.Glob(filepath.Join(e.opts.EvolveDir, "operator-salvage", "*", salvageBundleFile))
	if err != nil {
		e.t.Fatal(err)
	}
	return found
}

func (e *branchSweepEnv) failing(match func(args []string) bool) {
	boom := errors.New("injected git failure")
	e.opts.Exec = failingGitF1(sysexec.DefaultRunner, func(_ string, args []string) error {
		if match(args) {
			return boom
		}
		return nil
	})
}

func (e *branchSweepEnv) rewriting(match func(args []string) bool, stdout string) {
	e.opts.Exec = func(ctx context.Context, name, dir string, args, env []string, in io.Reader, out, errw io.Writer) (int, error) {
		if match(args) {
			_, err := io.WriteString(out, stdout)
			return 0, err
		}
		return sysexec.DefaultRunner(ctx, name, dir, args, env, in, out, errw)
	}
}

func branchSweepItem(m WorktreeManifest, branch string) (WorktreeItem, bool) {
	for _, it := range m.Items {
		if it.Branch == branch && it.Action == WorktreeActionDeleteBranch {
			return it, true
		}
	}
	return WorktreeItem{}, false
}

func TestBranchSweep_PlanAndApplyDeleteLandedAndEndedBranchesAfterBundles(t *testing.T) {
	e := newBranchSweepEnv(t)
	landedTip := e.land("feat/x", "x.txt")
	endedTip := e.end("feat/old", "old.txt")
	freshTip := e.commitOn("feat/y", "y.txt")
	e.repo.Git("branch", "cycle-9")

	m, err := PlanWorktrees(e.opts)
	if err != nil {
		t.Fatalf("PlanWorktrees: %v", err)
	}
	for branch, tip := range map[string]string{"feat/x": landedTip, "feat/old": endedTip} {
		it, ok := branchSweepItem(m, branch)
		if !ok || !it.Bundle || it.Head != tip {
			t.Errorf("%s not planned for a bundled delete at %s: %+v", branch, tip, m.Items)
		}
	}
	for _, branch := range []string{"feat/y", "main"} {
		if _, ok := branchSweepItem(m, branch); ok {
			t.Errorf("%s planned for delete: %+v", branch, m.Items)
		}
	}
	if it, ok := branchSweepItem(m, "cycle-9"); !ok || it.Bundle {
		t.Errorf("cycle-9 must keep the unbundled cycle delete: %+v", m.Items)
	}

	if err := ApplyWorktrees(e.opts, m); err != nil {
		t.Fatalf("ApplyWorktrees: %v", err)
	}
	for _, branch := range []string{"feat/x", "feat/old", "cycle-9"} {
		if got := e.tip(branch); got != "" {
			t.Errorf("%s survived the apply at %s", branch, got)
		}
	}
	if got := e.tip("feat/y"); got != freshTip {
		t.Errorf("feat/y moved to %q, want %s", got, freshTip)
	}
	if n := len(e.bundles()); n != 2 {
		t.Fatalf("%d bundle(s), want 2", n)
	}
	for _, b := range e.bundles() {
		if out, err := exec.Command("git", "-C", e.repo.Dir, "bundle", "verify", b).CombinedOutput(); err != nil {
			t.Errorf("bundle %s does not verify: %v\n%s", b, err, out)
		}
		if _, err := readSalvageLeaf(filepath.Dir(b)); err != nil {
			t.Errorf("salvage list cannot read %s: %v", filepath.Dir(b), err)
		}
	}
}

func TestBranchSweep_HookScopeAndMissingEndedLimitPlanNoOtherBranch(t *testing.T) {
	e := newBranchSweepEnv(t)
	e.land("feat/x", "x.txt")
	e.end("feat/old", "old.txt")

	hook := e.opts
	hook.BranchScope = BranchScopeCycle
	m, err := PlanWorktrees(hook)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Items) != 0 {
		t.Errorf("hook scope planned %+v", m.Items)
	}

	noLimit := e.opts
	noLimit.EndedAfter = 0
	m, err = PlanWorktrees(noLimit)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := branchSweepItem(m, "feat/old"); ok {
		t.Errorf("ended branch planned with no EndedAfter: %+v", m.Items)
	}
	if _, ok := branchSweepItem(m, "feat/x"); !ok {
		t.Errorf("landed branch not planned: %+v", m.Items)
	}
}

func TestBranchSweep_ACheckedOutBranchIsNeverPlanned(t *testing.T) {
	e := newBranchSweepEnv(t)
	e.land("feat/held", "h.txt")
	e.repo.Git("worktree", "add", "-q", filepath.Join(t.TempDir(), "held"), "feat/held")

	m, err := PlanWorktrees(e.opts)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := branchSweepItem(m, "feat/held"); ok {
		t.Errorf("a branch checked out in a worktree was planned: %+v", m.Items)
	}
}

func TestBranchSweep_PlanStepFailuresAbortThePlan(t *testing.T) {
	cases := []struct {
		name  string
		match func(args []string) bool
	}{
		{"worktree list", func(a []string) bool { return len(a) > 1 && a[0] == "worktree" && a[1] == "list" }},
		{"merged branches", func(a []string) bool { return len(a) > 1 && a[0] == "branch" && a[1] == "--merged" }},
		{"cycle branch listing", func(a []string) bool { return len(a) == 3 && a[0] == "branch" && a[1] == "--list" }},
		{"worktree status", func(a []string) bool { return len(a) > 0 && a[0] == "status" }},
		{"other branch listing", func(a []string) bool { return len(a) == 2 && a[0] == "branch" && a[1] == "--list" }},
		{"landing check", func(a []string) bool { return slices.Contains(a, "--name-only") }},
		{"tip read", func(a []string) bool {
			return len(a) > 0 && a[0] == "rev-parse" && slices.Contains(a, "refs/heads/feat/old")
		}},
		{"commit time", func(a []string) bool { return len(a) > 0 && a[0] == "log" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newBranchSweepEnv(t)
			e.end("feat/old", "old.txt")
			e.repo.Git("worktree", "add", "-q", filepath.Join(e.opts.WorktreeBase, "cycle-aaa-1"), "-b", "cycle-aaa-1")
			e.failing(tc.match)
			if m, err := PlanWorktrees(e.opts); err == nil || len(m.Items) != 0 {
				t.Fatalf("PlanWorktrees = %+v, %v; want an error and no items", m, err)
			}
		})
	}
}

func TestBranchSweep_AnUnreadableCommitTimeAbortsThePlan(t *testing.T) {
	e := newBranchSweepEnv(t)
	e.end("feat/old", "old.txt")
	e.rewriting(func(a []string) bool { return len(a) > 0 && a[0] == "log" }, "not-a-time\n")
	if _, err := PlanWorktrees(e.opts); err == nil {
		t.Fatal("PlanWorktrees accepted an unreadable commit time")
	}
}

func TestBranchSweep_ApplySkipsAnItemWhoseTargetWasRefused(t *testing.T) {
	e := newBranchSweepEnv(t)
	tip := e.land("feat/x", "x.txt")
	m := WorktreeManifest{Items: []WorktreeItem{
		{Path: filepath.Join(t.TempDir(), "gone"), Branch: "feat/x", Head: tip, Action: WorktreeActionDeleteBranch, Bundle: true},
		{Branch: "feat/x", Action: WorktreeActionFlagUnmerged},
	}}
	if err := ApplyWorktrees(e.opts, m); err == nil {
		t.Fatal("ApplyWorktrees reported no refusal for an unreadable target")
	}
	if got := e.tip("feat/x"); got != tip {
		t.Errorf("a refused item deleted the branch: tip %q", got)
	}
}

func TestBranchSweep_ABundledDeleteIsRefusedAndKeepsTheBranch(t *testing.T) {
	cases := []struct {
		name  string
		setup func(e *branchSweepEnv, it *WorktreeItem)
	}{
		{"tip moved since the plan", func(_ *branchSweepEnv, it *WorktreeItem) { it.Head = strings.Repeat("a", 40) }},
		{"worktree list fails", func(e *branchSweepEnv, _ *WorktreeItem) {
			e.failing(func(a []string) bool { return len(a) > 1 && a[0] == "worktree" && a[1] == "list" })
		}},
		{"checked out in a worktree", func(e *branchSweepEnv, it *WorktreeItem) {
			e.repo.Git("worktree", "add", "-q", filepath.Join(t.TempDir(), "held"), it.Branch)
		}},
		{"salvage store is a file", func(e *branchSweepEnv, _ *WorktreeItem) {
			if err := os.MkdirAll(e.opts.EvolveDir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(e.opts.EvolveDir, "operator-salvage"), []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{"bundle create fails", func(e *branchSweepEnv, _ *WorktreeItem) {
			e.failing(func(a []string) bool { return len(a) > 1 && a[0] == "bundle" && a[1] == "create" })
		}},
		{"bundle verify fails", func(e *branchSweepEnv, _ *WorktreeItem) {
			e.failing(func(a []string) bool { return len(a) > 1 && a[0] == "bundle" && a[1] == "verify" })
		}},
		{"bundle heads unreadable", func(e *branchSweepEnv, _ *WorktreeItem) {
			e.failing(func(a []string) bool { return len(a) > 1 && a[0] == "bundle" && a[1] == "list-heads" })
		}},
		{"bundle does not hold the tip", func(e *branchSweepEnv, _ *WorktreeItem) {
			e.rewriting(func(a []string) bool { return len(a) > 1 && a[0] == "bundle" && a[1] == "list-heads" }, "deadbeef refs/heads/other\n")
		}},
		{"bundle HEAD note cannot be written", func(e *branchSweepEnv, it *WorktreeItem) {
			noteDir := filepath.Join(e.opts.EvolveDir, "operator-salvage", "branch-feat-x-"+it.Head[:12], salvageHeadFile)
			if err := os.MkdirAll(noteDir, 0o755); err != nil {
				t.Fatal(err)
			}
		}},
		{"ref delete fails", func(e *branchSweepEnv, _ *WorktreeItem) {
			e.failing(func(a []string) bool { return len(a) > 0 && a[0] == "update-ref" })
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newBranchSweepEnv(t)
			tip := e.land("feat/x", "x.txt")
			it := WorktreeItem{Branch: "feat/x", Head: tip, Action: WorktreeActionDeleteBranch, Bundle: true}
			tc.setup(e, &it)
			err := ApplyWorktrees(e.opts, WorktreeManifest{Items: []WorktreeItem{it}})
			if err == nil {
				t.Fatal("ApplyWorktrees reported no error")
			}
			if tc.name != "ref delete fails" && e.tip("feat/x") != tip {
				t.Errorf("the branch was deleted although %s", tc.name)
			}
		})
	}
}

func TestBranchSweep_ApplyKeepsTheHistoricalDeleteOfASquashLandedBranch(t *testing.T) {
	setup := func(t *testing.T) (*branchSweepEnv, string) {
		e := newBranchSweepEnv(t)
		e.repo.Git("switch", "-q", "-c", "cycle-sq")
		commitRepoFile(t, e.repo, "sq.txt", "sq\n")
		tip := e.repo.Git("rev-parse", "HEAD")
		squashMerge(e.repo, "cycle-sq")
		return e, tip
	}
	manifest := func(tip string) WorktreeManifest {
		return WorktreeManifest{Items: []WorktreeItem{{Branch: "cycle-sq", Head: tip, Action: WorktreeActionDeleteBranch}}}
	}

	t.Run("deletes the squash-landed branch", func(t *testing.T) {
		e, tip := setup(t)
		if err := ApplyWorktrees(e.opts, manifest(tip)); err != nil {
			t.Fatal(err)
		}
		if e.tip("cycle-sq") != "" {
			t.Error("cycle-sq survived")
		}
	})
	t.Run("worktree list fails", func(t *testing.T) {
		e, tip := setup(t)
		e.failing(func(a []string) bool { return len(a) > 1 && a[0] == "worktree" && a[1] == "list" })
		if err := ApplyWorktrees(e.opts, manifest(tip)); err == nil {
			t.Fatal("no error")
		}
	})
	t.Run("squash check fails", func(t *testing.T) {
		e, tip := setup(t)
		e.failing(func(a []string) bool { return slices.Contains(a, "--name-only") })
		if err := ApplyWorktrees(e.opts, manifest(tip)); err == nil {
			t.Fatal("no error")
		}
	})
	t.Run("squash check no longer passes", func(t *testing.T) {
		e, tip := setup(t)
		e.repo.Git("switch", "-q", "cycle-sq")
		commitRepoFile(t, e.repo, "after.txt", "after\n")
		tip = e.repo.Git("rev-parse", "HEAD")
		e.repo.Git("switch", "-q", "main")
		err := ApplyWorktrees(e.opts, manifest(tip))
		if err == nil || !strings.Contains(err.Error(), "no longer passes") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("ref delete fails", func(t *testing.T) {
		e, tip := setup(t)
		e.failing(func(a []string) bool { return len(a) > 0 && a[0] == "update-ref" })
		if err := ApplyWorktrees(e.opts, manifest(tip)); err == nil {
			t.Fatal("no error")
		}
	})
}

func TestBranchSweep_PlanRefusesToRunWithoutAnExecSeam(t *testing.T) {
	if _, err := PlanWorktrees(WorktreeOptions{BranchScope: BranchScopeAll}); err == nil {
		t.Fatal("PlanWorktrees ran without Exec")
	}
}

func TestBranchSweep_TheScopeTypeDefaultsToTheCycleScope(t *testing.T) {
	var scope BranchScope
	if scope != BranchScopeCycle || scope == BranchScopeAll {
		t.Fatalf("zero BranchScope = %d, want BranchScopeCycle", scope)
	}
}
