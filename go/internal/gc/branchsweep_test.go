package gc

import (
	"bytes"
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

var errInjectedGit = errors.New("injected git failure")

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
	e.opts.Exec = failingGitF1(sysexec.DefaultRunner, func(_ string, args []string) error {
		if match(args) {
			return errInjectedGit
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

func branchSweepPlanItem(m BranchSweepManifest, side BranchSide, branch string) (BranchSweepItem, bool) {
	for _, it := range m.Items {
		if it.Side == side && it.Branch == branch && it.Action == BranchActionDelete {
			return it, true
		}
	}
	return BranchSweepItem{}, false
}

func TestBranchSweep_PlanAndApplyDeleteLandedAndPRBranchesWithRecords(t *testing.T) {
	e := newBranchSweepEnv(t)
	origin := gittest.Bare(t)
	e.repo.Git("remote", "add", "origin", origin.Dir)
	e.repo.Git("push", "-q", "origin", "main")

	b2Tip := e.land("console/landed", "x.txt")
	e.repo.Git("push", "-q", "origin", "main")
	mergedTip := e.commitOn("console/merged-pr", "m.txt")
	e.repo.Git("push", "-q", "origin", "console/merged-pr")
	unownedTip := e.land("feat/unowned", "u.txt")
	e.repo.Git("push", "-q", "origin", "main")
	e.repo.Git("branch", "console/empty-new", "main")
	e.repo.Git("branch", "cycle-9")

	api := &apiRS{prs: []RemotePR{{Number: 7, Branch: "console/merged-pr", State: "MERGED", HeadRefOid: mergedTip}}}
	var log bytes.Buffer
	opts := BranchSweepOptions{
		ProjectRoot: e.repo.Dir,
		EvolveDir:   e.opts.EvolveDir,
		Remote:      "origin",
		Stage:       RemoteStageEnforce,
		Exec:        sysexec.DefaultRunner,
		API:         api,
		Log:         &log,
	}

	m, err := PlanBranchSweep(context.Background(), opts)
	if err != nil {
		t.Fatalf("PlanBranchSweep: %v", err)
	}
	if it, ok := branchSweepPlanItem(m, BranchSideLocal, "console/landed"); !ok || it.Class != "B2" || it.Tip != b2Tip {
		t.Errorf("console/landed local item = %+v (ok=%v), want B2 at %s", it, ok, b2Tip)
	}
	if it, ok := branchSweepPlanItem(m, BranchSideLocal, "console/merged-pr"); !ok || it.Class != "B1" || it.Tip != mergedTip {
		t.Errorf("console/merged-pr local item = %+v (ok=%v), want B1", it, ok)
	}
	if it, ok := branchSweepPlanItem(m, BranchSideRemote, "console/merged-pr"); !ok || it.Class != "B1" || it.Tip != mergedTip {
		t.Errorf("console/merged-pr remote item = %+v (ok=%v), want B1", it, ok)
	}
	for _, kept := range []string{"feat/unowned", "console/empty-new", "cycle-9", "main"} {
		if _, ok := branchSweepPlanItem(m, BranchSideLocal, kept); ok {
			t.Errorf("%s planned for delete: %+v", kept, m.Items)
		}
	}

	if err := ApplyBranchSweep(context.Background(), opts, m); err != nil {
		t.Fatalf("ApplyBranchSweep: %v", err)
	}
	for _, gone := range []string{"console/landed", "console/merged-pr"} {
		if got := e.tip(gone); got != "" {
			t.Errorf("%s survived apply at %s", gone, got)
		}
	}
	if got := e.tip("feat/unowned"); got != unownedTip {
		t.Errorf("feat/unowned moved to %q, want %s", got, unownedTip)
	}
	if len(api.deleted) != 1 || api.deleted[0] != "console/merged-pr" {
		t.Errorf("remote deleted = %v, want [console/merged-pr]", api.deleted)
	}
}

func TestBranchSweep_HookScopePlansNoOtherBranch(t *testing.T) {
	e := newBranchSweepEnv(t)
	e.land("console/landed", "x.txt")
	hook := e.opts
	hook.BranchScope = BranchScopeCycle
	m, err := PlanWorktrees(hook)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Items) != 0 {
		t.Errorf("PlanWorktrees planned non-cycle branch: %+v", m.Items)
	}
}

func TestBranchSweep_ACheckedOutBranchIsNeverPlanned(t *testing.T) {
	e := newBranchSweepEnv(t)
	origin := gittest.Bare(t)
	e.repo.Git("remote", "add", "origin", origin.Dir)
	e.repo.Git("push", "-q", "origin", "main")
	tip := e.land("console/held", "h.txt")
	e.repo.Git("push", "-q", "origin", "main")
	e.repo.Git("worktree", "add", "-q", filepath.Join(t.TempDir(), "held"), "console/held")

	opts := BranchSweepOptions{
		ProjectRoot: e.repo.Dir,
		EvolveDir:   e.opts.EvolveDir,
		Remote:      "origin",
		Exec:        sysexec.DefaultRunner,
		API:         &apiRS{prs: []RemotePR{{Number: 9, Branch: "console/held", State: "MERGED", HeadRefOid: tip}}},
	}
	m, err := PlanBranchSweep(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := branchSweepPlanItem(m, BranchSideLocal, "console/held"); ok {
		t.Errorf("checked-out branch was planned: %+v", m.Items)
	}
}

func TestBranchSweep_PlanStepFailuresAbortThePlan(t *testing.T) {
	cases := []struct {
		name  string
		match func(args []string) bool
	}{
		{"worktree list", func(a []string) bool { return len(a) > 1 && a[0] == "worktree" && a[1] == "list" }},
		{"for-each-ref", func(a []string) bool { return len(a) > 0 && a[0] == "for-each-ref" }},
		{"ls-remote", func(a []string) bool { return len(a) > 0 && a[0] == "ls-remote" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newBranchSweepEnv(t)
			e.commitOn("console/x", "x.txt")
			e.failing(tc.match)
			opts := BranchSweepOptions{
				ProjectRoot: e.repo.Dir,
				EvolveDir:   e.opts.EvolveDir,
				Exec:        e.opts.Exec,
				API:         &apiRS{},
			}
			if m, err := PlanBranchSweep(context.Background(), opts); err == nil || len(m.Items) != 0 {
				t.Fatalf("PlanBranchSweep = %+v, %v; want error", m, err)
			}
		})
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
		e, _ := setup(t)
		e.repo.Git("switch", "-q", "cycle-sq")
		commitRepoFile(t, e.repo, "after.txt", "after\n")
		tip := e.repo.Git("rev-parse", "HEAD")
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
	if _, err := PlanBranchSweep(context.Background(), BranchSweepOptions{}); err == nil {
		t.Fatal("PlanBranchSweep ran without Exec")
	}
}

func TestBranchSweep_TheScopeTypeDefaultsToTheCycleScope(t *testing.T) {
	var scope BranchScope
	if scope != BranchScopeCycle || scope == BranchScopeAll {
		t.Fatalf("zero BranchScope = %d, want BranchScopeCycle", scope)
	}
}

func TestBranchSweep_AFailedBundleRefusalWrapsErrBranchKeptAndTheCause(t *testing.T) {
	e := newBranchSweepEnv(t)
	tip := e.land("feat/x", "x.txt")
	e.failing(func(a []string) bool { return len(a) > 1 && a[0] == "bundle" && a[1] == "create" })
	it := WorktreeItem{Branch: "feat/x", Head: tip, Action: WorktreeActionDeleteBranch, Bundle: true}

	err := ApplyWorktrees(e.opts, WorktreeManifest{Items: []WorktreeItem{it}})

	if !errors.Is(err, ErrBranchKept) {
		t.Errorf("err = %v, want it to wrap ErrBranchKept", err)
	}
	if !errors.Is(err, errInjectedGit) {
		t.Errorf("err = %v, want it to wrap the bundle failure", err)
	}
}
