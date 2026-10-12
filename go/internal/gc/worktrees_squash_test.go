package gc

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type cycleSquashEnv struct {
	t    *testing.T
	repo *gittest.Repo
	base string
}

func newCycleSquashEnv(t *testing.T) cycleSquashEnv {
	t.Helper()
	r := newSquashRepo(t)
	base := filepath.Join(r.Dir, ".evolve", "worktrees")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	return cycleSquashEnv{t: t, repo: r, base: base}
}

func (e cycleSquashEnv) options() WorktreeOptions {
	return WorktreeOptions{ProjectRoot: e.repo.Dir, WorktreeBase: e.base, EvolveDir: filepath.Join(e.repo.Dir, ".evolve"), Exec: sysexec.DefaultRunner}
}

func (e cycleSquashEnv) landedCycle(leaf, file string) string {
	e.t.Helper()
	wt := filepath.Join(e.base, leaf)
	e.repo.Git("worktree", "add", "-q", "-b", leaf, wt, "main")
	writeRepoFile(e.t, wt, file, leaf+"\n")
	e.repo.Git("-C", wt, "add", "-A")
	e.repo.Git("-C", wt, "commit", "-q", "-m", "work "+leaf)
	squashMerge(e.repo, leaf)
	return wt
}

func itemsByAction(m WorktreeManifest, a WorktreeAction) []WorktreeItem {
	var out []WorktreeItem
	for _, it := range m.Items {
		if it.Action == a {
			out = append(out, it)
		}
	}
	return out
}

func TestPlanWorktrees_ASquashMergedCycleTreeAndItsBranchAreSweptEndToEnd(t *testing.T) {
	e := newCycleSquashEnv(t)
	wt := e.landedCycle("cycle-sq-910", "sq.txt")
	o := e.options()

	m, err := PlanWorktrees(o)
	if err != nil {
		t.Fatal(err)
	}
	removes, deletes := itemsByAction(m, WorktreeActionRemove), itemsByAction(m, WorktreeActionDeleteBranch)
	if len(removes) != 1 || removes[0].Path != wt || removes[0].Reason != "squash-merged, clean, dead" {
		t.Fatalf("remove items = %+v, want one squash-merged remove of %s", removes, wt)
	}
	wantHead := e.repo.Git("rev-parse", "refs/heads/cycle-sq-910")
	if len(deletes) != 1 || deletes[0].Head != wantHead {
		t.Fatalf("delete-branch items = %+v, want one carrying head %s", deletes, wantHead)
	}
	if flags := itemsByAction(m, WorktreeActionFlagUnmerged); len(flags) != 0 {
		t.Fatalf("a squash-merged branch was flagged unmerged: %+v", flags)
	}

	if err := ApplyWorktrees(o, m); err != nil {
		t.Fatalf("ApplyWorktrees: %v", err)
	}
	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Errorf("worktree %s still exists after apply (stat err %v)", wt, err)
	}
	if got := e.repo.Git("branch", "--list", "cycle-sq-910"); got != "" {
		t.Errorf("branch cycle-sq-910 survived apply: %q", got)
	}
}

func TestPlanWorktrees_ABranchWithCommitsAfterTheSquashIsFlaggedAndKept(t *testing.T) {
	e := newCycleSquashEnv(t)
	wt := e.landedCycle("cycle-sq-911", "sq.txt")
	writeRepoFile(t, wt, "after.txt", "work after the squash\n")
	e.repo.Git("-C", wt, "add", "-A")
	e.repo.Git("-C", wt, "commit", "-q", "-m", "after the squash")

	m, err := PlanWorktrees(e.options())
	if err != nil {
		t.Fatal(err)
	}
	flags := itemsByAction(m, WorktreeActionFlagUnmerged)
	if len(flags) != 1 || flags[0].Path != wt {
		t.Fatalf("flag-unmerged items = %+v, want %s flagged", flags, wt)
	}
	if len(itemsByAction(m, WorktreeActionRemove))+len(itemsByAction(m, WorktreeActionDeleteBranch)) != 0 {
		t.Fatalf("a branch with unlanded commits was planned for removal: %+v", m.Items)
	}
}

func TestPlanWorktrees_ASquashMergedOrphanBranchIsDeletedByItsHeadAndAnUnlandedOneIsFlagged(t *testing.T) {
	e := newCycleSquashEnv(t)
	e.repo.Git("switch", "-q", "-c", "cycle-orph-920")
	commitRepoFile(t, e.repo, "orph.txt", "orph\n")
	squashMerge(e.repo, "cycle-orph-920")
	e.repo.Git("switch", "-q", "-c", "cycle-open-921")
	commitRepoFile(t, e.repo, "open.txt", "still open\n")
	e.repo.Git("switch", "-q", "main")
	o := e.options()

	m, err := PlanWorktrees(o)
	if err != nil {
		t.Fatal(err)
	}
	deletes, flags := itemsByAction(m, WorktreeActionDeleteBranch), itemsByAction(m, WorktreeActionFlagUnmerged)
	if len(deletes) != 1 || deletes[0].Branch != "cycle-orph-920" || deletes[0].Head == "" || deletes[0].Reason != "squash-merged orphan branch (no worktree)" {
		t.Fatalf("delete-branch items = %+v, want the squash-merged orphan with its head", deletes)
	}
	if len(flags) != 1 || flags[0].Branch != "cycle-open-921" {
		t.Fatalf("flag-unmerged items = %+v, want only cycle-open-921", flags)
	}
	if err := ApplyWorktrees(o, m); err != nil {
		t.Fatal(err)
	}
	if got := e.repo.Git("branch", "--list", "cycle-orph-920", "cycle-open-921"); got == "" || got == "cycle-orph-920" {
		t.Errorf("branches after apply = %q, want only cycle-open-921", got)
	}
}

func TestApplyWorktrees_ASquashMergedBranchThatMovedAfterThePlanIsNotDeleted(t *testing.T) {
	e := newCycleSquashEnv(t)
	e.repo.Git("switch", "-q", "-c", "cycle-move-930")
	commitRepoFile(t, e.repo, "move.txt", "move\n")
	squashMerge(e.repo, "cycle-move-930")
	o := e.options()
	m, err := PlanWorktrees(o)
	if err != nil {
		t.Fatal(err)
	}
	e.repo.Git("switch", "-q", "cycle-move-930")
	commitRepoFile(t, e.repo, "late.txt", "late work\n")
	e.repo.Git("switch", "-q", "main")

	if err := ApplyWorktrees(o, m); err == nil {
		t.Fatal("ApplyWorktrees = nil, want an error for the branch that moved after the plan")
	}
	if got := e.repo.Git("branch", "--list", "cycle-move-930"); got == "" {
		t.Error("the branch that gained a commit after the plan was deleted")
	}
}

func TestPlanWorktrees_AGitFailureWhileJudgingASquashSurfaces(t *testing.T) {
	e := newCycleSquashEnv(t)
	e.repo.Git("switch", "-q", "-c", "cycle-err-940")
	commitRepoFile(t, e.repo, "err.txt", "err\n")
	e.repo.Git("switch", "-q", "main")
	o := e.options()
	o.Exec = func(ctx context.Context, name, dir string, args, env []string, in io.Reader, out, errw io.Writer) (int, error) {
		if len(args) > 0 && args[0] == "merge-base" {
			return 128, nil
		}
		return sysexec.DefaultRunner(ctx, name, dir, args, env, in, out, errw)
	}
	if _, err := PlanWorktrees(o); err == nil {
		t.Fatal("PlanWorktrees = nil, want the merge-base failure")
	}
}

func TestPlanWorktrees_AFailureReadingASquashedBranchHeadSurfaces(t *testing.T) {
	e := newCycleSquashEnv(t)
	e.landedCycle("cycle-head-950", "head.txt")
	o := e.options()
	o.Exec = failOn("rev-parse", 128, nil)
	if _, err := PlanWorktrees(o); err == nil {
		t.Fatal("PlanWorktrees = nil, want the rev-parse failure for the squash-merged worktree branch")
	}
}

func TestPlanWorktrees_AFailureReadingASquashedOrphanBranchHeadSurfaces(t *testing.T) {
	e := newCycleSquashEnv(t)
	e.repo.Git("switch", "-q", "-c", "cycle-orphhead-951")
	commitRepoFile(t, e.repo, "oh.txt", "oh\n")
	squashMerge(e.repo, "cycle-orphhead-951")
	o := e.options()
	o.Exec = failOn("rev-parse", 128, nil)
	if _, err := PlanWorktrees(o); err == nil {
		t.Fatal("PlanWorktrees = nil, want the rev-parse failure for the squash-merged orphan branch")
	}
}

func TestPlanWorktrees_AFailureJudgingASquashForAWorktreeBranchSurfaces(t *testing.T) {
	e := newCycleSquashEnv(t)
	e.landedCycle("cycle-judge-952", "judge.txt")
	o := e.options()
	o.Exec = failOn("merge-base", 128, nil)
	if _, err := PlanWorktrees(o); err == nil {
		t.Fatal("PlanWorktrees = nil, want the merge-base failure")
	}
}

func TestMergedSet_AMemoisedVerdictCostsNoSecondGitCall(t *testing.T) {
	e := newCycleSquashEnv(t)
	e.repo.Git("switch", "-q", "-c", "cycle-memo-953")
	commitRepoFile(t, e.repo, "memo.txt", "memo\n")
	squashMerge(e.repo, "cycle-memo-953")
	calls := 0
	o := e.options()
	o.Exec = func(ctx context.Context, name, dir string, args, env []string, in io.Reader, out, errw io.Writer) (int, error) {
		if len(args) > 0 && args[0] == "merge-base" {
			calls++
		}
		return sysexec.DefaultRunner(ctx, name, dir, args, env, in, out, errw)
	}
	set, err := o.mergedBranches()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if l, err := set.landing("cycle-memo-953"); err != nil || l != landingSquash {
			t.Fatalf("landing = %v, %v; want squash", l, err)
		}
	}
	if calls != 1 {
		t.Errorf("merge-base ran %d times for three verdicts, want 1", calls)
	}
}

func TestMergedBranches_AFailingMergedListSurfaces(t *testing.T) {
	e := newCycleSquashEnv(t)
	o := e.options()
	o.Exec = failOn("branch", 128, nil)
	if _, err := o.mergedBranches(); err == nil {
		t.Fatal("mergedBranches = nil error, want the git branch --merged failure")
	}
}

func TestPlanWorktrees_AFailingBranchListSurfaces(t *testing.T) {
	e := newCycleSquashEnv(t)
	o := e.options()
	o.Exec = func(ctx context.Context, name, dir string, args, env []string, in io.Reader, out, errw io.Writer) (int, error) {
		if len(args) > 1 && args[0] == "branch" && args[1] == "--list" {
			return 128, nil
		}
		return sysexec.DefaultRunner(ctx, name, dir, args, env, in, out, errw)
	}
	if _, err := PlanWorktrees(o); err == nil {
		t.Fatal("PlanWorktrees = nil, want the git branch --list failure")
	}
}
