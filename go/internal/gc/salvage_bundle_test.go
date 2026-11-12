package gc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type salvageBundleEnv struct {
	repo     *gittest.Repo
	tree     string
	branch   string
	tip      string
	options  WorktreeOptions
	manifest WorktreeManifest
}

func newSalvageBundleEnv(t *testing.T) salvageBundleEnv {
	t.Helper()
	repo := gittest.Fixture(t)
	writeFile(t, filepath.Join(repo.Dir, "f.txt"), "base\n")
	repo.Git("add", "f.txt")
	repo.Git("commit", "-q", "-m", "base")
	base := filepath.Join(repo.Dir, ".evolve", "worktrees")
	branch := "cycle-cd3ae73e-1677"
	tree := filepath.Join(base, branch)
	repo.Git("worktree", "add", "-q", tree, "-b", branch)
	writeFile(t, filepath.Join(tree, "f.txt"), "base\nlane edit\n")
	repo.Git("-C", tree, "commit", "-q", "-am", "lane work")
	writeFile(t, filepath.Join(tree, "f.txt"), "base\nlane edit\nuncommitted edit\n")
	now := time.Now()
	closeOutCycle(t, repo.Dir, 1677, now.Add(-30*time.Hour))
	o := WorktreeOptions{ProjectRoot: repo.Dir, WorktreeBase: base, EvolveDir: filepath.Join(repo.Dir, ".evolve"),
		Policy: WorktreesPolicy{SalvageAfterHours: 24}, Now: func() time.Time { return now }, Exec: sysexec.DefaultRunner}
	m, err := PlanWorktrees(o)
	if err != nil {
		t.Fatal(err)
	}
	if got := actionFor(m.Items, tree); got != WorktreeActionSalvageRemove {
		t.Fatalf("planned %q, want %q", got, WorktreeActionSalvageRemove)
	}
	return salvageBundleEnv{repo: repo, tree: tree, branch: branch, tip: repo.Git("rev-parse", branch), options: o, manifest: m}
}

func (e salvageBundleEnv) leaf() string {
	return filepath.Join(OperatorSalvageDir(e.options.EvolveDir), filepath.Base(e.tree))
}

func requireVerifiedBundle(t *testing.T, repo *gittest.Repo, bundle, branch, tip string) {
	t.Helper()
	if _, err := os.Stat(bundle); err != nil {
		t.Fatalf("the salvage leaf has no bundle: %v", err)
	}
	repo.Git("bundle", "verify", bundle)
	if heads := repo.Git("bundle", "list-heads", bundle); !strings.Contains(heads, tip+" refs/heads/"+branch) {
		t.Errorf("bundle heads %q do not hold %s at %s", heads, branch, tip)
	}
}

func TestApplyWorktrees_ASalvageRemovalStoresAVerifiedBundleOfTheLaneBranch(t *testing.T) {
	e := newSalvageBundleEnv(t)

	if err := ApplyWorktrees(e.options, e.manifest); err != nil {
		t.Fatalf("ApplyWorktrees: %v", err)
	}

	requireVerifiedBundle(t, e.repo, filepath.Join(e.leaf(), "refs.bundle"), e.branch, e.tip)
	if head := readFile(t, filepath.Join(e.leaf(), "HEAD")); !strings.HasPrefix(head, e.tip) {
		t.Errorf("salvage HEAD %q does not record the tip %s", head, e.tip)
	}
	leaves, err := ListSalvage(e.options.EvolveDir)
	if err != nil || len(leaves) != 1 || leaves[0].Head != e.tip {
		t.Errorf("ListSalvage = %+v, %v; want the one leaf at the tip", leaves, err)
	}
}

func TestApplyWorktrees_ASalvageWhoseBundleStepFailsKeepsTheTreeAndTheBranch(t *testing.T) {
	for _, step := range []string{"create", "verify"} {
		t.Run(step, func(t *testing.T) {
			e := newSalvageBundleEnv(t)
			gittest.InstallFault(t, gittest.GitFault{Verb: "bundle", Args: []string{step}, Code: 1})

			err := ApplyWorktrees(e.options, e.manifest)

			if err == nil || !strings.Contains(err.Error(), "tree kept") {
				t.Fatalf("ApplyWorktrees err = %v, want a refusal that names the kept tree", err)
			}
			if _, statErr := os.Stat(e.tree); statErr != nil {
				t.Errorf("the tree is gone after a failed bundle %s: %v", step, statErr)
			}
			if !gittest.BranchExists(t, e.repo.Dir, e.branch) {
				t.Errorf("the branch %s is gone after a failed bundle %s", e.branch, step)
			}
			if got := readFile(t, filepath.Join(e.tree, "f.txt")); got != "base\nlane edit\nuncommitted edit\n" {
				t.Errorf("the uncommitted edit is %q, want it untouched in place", got)
			}
		})
	}
}

func TestApplyDevWorktrees_TheArchiveWritesTheSalvageLeafWithAVerifiedBundle(t *testing.T) {
	e := newDevEnv(t)
	dir := e.addDev("layout")
	e.commitIn(dir, "work.txt", "committed\n")
	writeRepoFile(t, dir, "base.txt", "edited tracked\n")
	writeRepoFile(t, dir, "deep/untracked.txt", "untracked work\n")
	tip := e.store.Git("rev-parse", "br-layout")
	o := e.options(idleOffset)

	recs, err := e.apply(o, e.plan(o))
	if err != nil || len(recs) != 1 {
		t.Fatalf("apply = %+v, %v", recs, err)
	}

	leaf := filepath.Join(OperatorSalvageDir(e.evolve), "layout")
	requireVerifiedBundle(t, e.store, filepath.Join(leaf, "refs.bundle"), "br-layout", tip)
	if head := readFile(t, filepath.Join(leaf, "HEAD")); !strings.HasPrefix(head, tip+" br-layout") {
		t.Errorf("salvage HEAD %q does not record %s br-layout", head, tip)
	}
	if patch := readFile(t, filepath.Join(leaf, "uncommitted.patch")); !strings.Contains(patch, "edited tracked") {
		t.Errorf("uncommitted.patch %q does not hold the tracked edit", patch)
	}
	if got := untarFile(t, filepath.Join(leaf, "untracked.tgz"), "deep/untracked.txt"); got != "untracked work\n" {
		t.Errorf("untracked.tgz holds deep/untracked.txt = %q", got)
	}
}

func TestApplyDevWorktrees_ACleanTreeAlsoGetsAVerifiedBundleBeforeItsRemoval(t *testing.T) {
	e := newDevEnv(t)
	dir := e.addDev("cleanbundle")
	e.commitIn(dir, "work.txt", "committed\n")
	tip := e.store.Git("rev-parse", "br-cleanbundle")
	o := e.options(idleOffset)

	recs, err := e.apply(o, e.plan(o))
	if err != nil || len(recs) != 1 {
		t.Fatalf("apply = %+v, %v", recs, err)
	}

	requireVerifiedBundle(t, e.store, filepath.Join(OperatorSalvageDir(e.evolve), "cleanbundle", "refs.bundle"), "br-cleanbundle", tip)
}

func TestApplyDevWorktrees_AnArchiveWhoseBundleStepFailsKeepsTheTreeAndTheBranch(t *testing.T) {
	for _, step := range []string{"create", "verify"} {
		t.Run(step, func(t *testing.T) {
			e := newDevEnv(t)
			dir := e.addDev("unbundled")
			e.commitIn(dir, "work.txt", "committed\n")
			writeRepoFile(t, dir, "wip.txt", "uncommitted\n")
			o := e.options(idleOffset)
			m := e.plan(o)
			gittest.InstallFault(t, gittest.GitFault{Verb: "bundle", Args: []string{step}, Code: 1})

			recs, err := e.apply(o, m)

			if err == nil || !strings.Contains(err.Error(), "bundle") || !strings.Contains(err.Error(), "kept") {
				t.Fatalf("err = %v, want a refusal that names the bundle and the kept tree", err)
			}
			if len(recs) != 0 {
				t.Errorf("records = %+v, want none", recs)
			}
			if _, statErr := os.Stat(dir); statErr != nil {
				t.Errorf("the tree is gone after a failed bundle %s: %v", step, statErr)
			}
			if !e.branchExists("br-unbundled") {
				t.Errorf("the branch br-unbundled is gone after a failed bundle %s", step)
			}
			if stored, _ := ReadDevArchive(e.evolve); len(stored) != 0 {
				t.Errorf("manifest = %+v, want no record for a refused archive", stored)
			}
		})
	}
}
