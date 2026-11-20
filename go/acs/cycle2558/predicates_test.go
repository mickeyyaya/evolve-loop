//go:build acs

package cycle2558

import (
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const (
	bundleSizeLimit = 1 << 20
	bundleFileName  = "refs.bundle"
	headFileName    = "HEAD"
)

type sweepRepo struct {
	tb   *testing.T
	repo *gittest.Repo
	opts gc.WorktreeOptions
}

func writeRepoFile(t *testing.T, dir, rel string, body []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, rel), body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func newSweepRepo(t *testing.T, withOrigin bool) *sweepRepo {
	t.Helper()
	repo := gittest.Fixture(t)
	bulk := make([]byte, 3<<20)
	if _, err := rand.Read(bulk); err != nil {
		t.Fatal(err)
	}
	writeRepoFile(t, repo.Dir, "bulk.bin", bulk)
	repo.Git("add", "bulk.bin")
	repo.Git("commit", "-q", "-m", "bulk history")
	if withOrigin {
		repo.Git("update-ref", "refs/remotes/origin/main", "refs/heads/main")
	}
	return &sweepRepo{tb: t, repo: repo, opts: gc.WorktreeOptions{
		ProjectRoot:  repo.Dir,
		WorktreeBase: filepath.Join(repo.Dir, ".evolve", "worktrees"),
		EvolveDir:    filepath.Join(repo.Dir, ".evolve"),
		Exec:         sysexec.DefaultRunner,
		BranchScope:  gc.BranchScopeAll,
	}}
}

func (s *sweepRepo) commitOn(branch, file string) string {
	s.repo.Git("checkout", "-q", "-b", branch)
	writeRepoFile(s.t(), s.repo.Dir, file, []byte(branch+"\n"))
	s.repo.Git("add", file)
	s.repo.Git("commit", "-q", "-m", "work "+file)
	tip := s.repo.Git("rev-parse", "HEAD")
	s.repo.Git("checkout", "-q", "main")
	return tip
}

func (s *sweepRepo) t() *testing.T { return s.tb }

func (s *sweepRepo) landInOriginMain(branch, file string) string {
	tip := s.commitOn(branch, file)
	s.repo.Git("merge", "-q", "--no-ff", "-m", "land "+branch, branch)
	s.repo.Git("update-ref", "refs/remotes/origin/main", "refs/heads/main")
	return tip
}

func (s *sweepRepo) sweepBranch(branch, tip string) error {
	item := gc.WorktreeItem{Branch: branch, Head: tip, Action: gc.WorktreeActionDeleteBranch, Bundle: true}
	return gc.ApplyWorktrees(s.opts, gc.WorktreeManifest{Items: []gc.WorktreeItem{item}})
}

func (s *sweepRepo) leaves() []string {
	dirs, err := filepath.Glob(filepath.Join(gc.OperatorSalvageDir(s.opts.EvolveDir), "*"))
	if err != nil {
		s.t().Fatal(err)
	}
	return dirs
}

func (s *sweepRepo) addLaneTree(branch string) (tree, tip string) {
	tree = filepath.Join(s.opts.WorktreeBase, branch)
	s.repo.Git("worktree", "add", "-q", tree, "-b", branch)
	writeRepoFile(s.t(), tree, "lane.txt", []byte("lane work\n"))
	s.repo.Git("-C", tree, "add", "lane.txt")
	s.repo.Git("-C", tree, "commit", "-q", "-m", "lane work")
	writeRepoFile(s.t(), tree, "wip.txt", []byte("uncommitted\n"))
	return tree, s.repo.Git("rev-parse", branch)
}

func (s *sweepRepo) salvageTree(tree, branch string) error {
	item := gc.WorktreeItem{Path: tree, Branch: branch, Action: gc.WorktreeActionSalvageRemove}
	return gc.ApplyWorktrees(s.opts, gc.WorktreeManifest{Items: []gc.WorktreeItem{item}})
}

func requireThinVerifiedBundle(t *testing.T, repo *gittest.Repo, bundle, branch, tip string) {
	t.Helper()
	info, err := os.Stat(bundle)
	if err != nil {
		t.Fatalf("RED: no bundle in the leaf: %v", err)
	}
	if info.Size() >= bundleSizeLimit {
		t.Errorf("RED: bundle is %d bytes, want under %d: it holds history that origin/main already has", info.Size(), bundleSizeLimit)
	}
	repo.Git("bundle", "verify", bundle)
	if heads := repo.Git("bundle", "list-heads", bundle); !strings.Contains(heads, tip+" refs/heads/"+branch) {
		t.Errorf("bundle heads %q do not hold %s at %s", heads, branch, tip)
	}
}

func TestC2558_001_BranchSweepLeafOneCommitPastOriginMainIsThinAndVerifies(t *testing.T) {
	s := newSweepRepo(t, true)
	tip := s.commitOn("lane/thin", "thin.txt")

	if err := s.sweepBranch("lane/thin", tip); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	leaves := s.leaves()
	if len(leaves) != 1 {
		t.Fatalf("leaves = %v, want one", leaves)
	}
	requireThinVerifiedBundle(t, s.repo, filepath.Join(leaves[0], bundleFileName), "lane/thin", tip)
	if gittest.BranchExists(t, s.repo.Dir, "lane/thin") {
		t.Error("the branch survived a sweep whose bundle was good")
	}
}

func TestC2558_002_SalvageLeafOfATreeOneCommitPastOriginMainIsThinAndVerifies(t *testing.T) {
	s := newSweepRepo(t, true)
	tree, tip := s.addLaneTree("cycle-thin-2558")

	if err := s.salvageTree(tree, "cycle-thin-2558"); err != nil {
		t.Fatalf("salvage: %v", err)
	}

	leaf := filepath.Join(gc.OperatorSalvageDir(s.opts.EvolveDir), filepath.Base(tree))
	requireThinVerifiedBundle(t, s.repo, filepath.Join(leaf, bundleFileName), "cycle-thin-2558", tip)
	if _, err := os.Stat(tree); !os.IsNotExist(err) {
		t.Errorf("the tree should be released after a good salvage, stat err = %v", err)
	}
}

func TestC2558_003_BranchSweepOfATipOriginMainHoldsWritesHeadAndNoBundleAndReleases(t *testing.T) {
	s := newSweepRepo(t, true)
	tip := s.landInOriginMain("lane/landed", "landed.txt")

	if err := s.sweepBranch("lane/landed", tip); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	leaves := s.leaves()
	if len(leaves) != 1 {
		t.Fatalf("leaves = %v, want one", leaves)
	}
	if _, err := os.Stat(filepath.Join(leaves[0], bundleFileName)); !os.IsNotExist(err) {
		t.Errorf("RED: a tip that origin/main holds got a bundle, stat err = %v", err)
	}
	head, err := os.ReadFile(filepath.Join(leaves[0], headFileName))
	if err != nil {
		t.Fatalf("leaf has no HEAD: %v", err)
	}
	if string(head) != tip+" lane/landed\n" {
		t.Errorf("leaf HEAD = %q, want the tip and the branch", head)
	}
	if gittest.BranchExists(t, s.repo.Dir, "lane/landed") {
		t.Error("the release did not go on: the landed branch survived")
	}
}

func TestC2558_004_SalvageOfATreeWhoseTipOriginMainHoldsWritesHeadAndNoBundleAndReleases(t *testing.T) {
	s := newSweepRepo(t, true)
	tree := filepath.Join(s.opts.WorktreeBase, "cycle-landed-2558")
	s.repo.Git("worktree", "add", "-q", tree, "-b", "cycle-landed-2558")
	tip := s.repo.Git("rev-parse", "cycle-landed-2558")
	writeRepoFile(t, tree, "wip.txt", []byte("uncommitted\n"))

	if err := s.salvageTree(tree, "cycle-landed-2558"); err != nil {
		t.Fatalf("salvage: %v", err)
	}

	leaf := filepath.Join(gc.OperatorSalvageDir(s.opts.EvolveDir), filepath.Base(tree))
	if _, err := os.Stat(filepath.Join(leaf, bundleFileName)); !os.IsNotExist(err) {
		t.Errorf("RED: a tip that origin/main holds got a bundle, stat err = %v", err)
	}
	head, err := os.ReadFile(filepath.Join(leaf, headFileName))
	if err != nil || !strings.HasPrefix(string(head), tip+" cycle-landed-2558") {
		t.Fatalf("leaf HEAD = %q, %v; want the tip and the branch", head, err)
	}
	if _, err := os.Stat(tree); !os.IsNotExist(err) {
		t.Errorf("the release did not go on: the tree is still there, stat err = %v", err)
	}
}

func TestC2558_005_FailedBundleStepOnABranchSweepKeepsTheBranch(t *testing.T) {
	for _, step := range []string{"create", "verify"} {
		t.Run(step, func(t *testing.T) {
			s := newSweepRepo(t, true)
			tip := s.commitOn("lane/keep", "keep.txt")
			gittest.InstallFault(t, gittest.GitFault{Verb: "bundle", Args: []string{step}, Code: 1})

			err := s.sweepBranch("lane/keep", tip)

			if err == nil || !errors.Is(err, gc.ErrBranchKept) {
				t.Fatalf("sweep err = %v, want a refusal that names the kept branch", err)
			}
			if !gittest.BranchExists(t, s.repo.Dir, "lane/keep") {
				t.Errorf("the branch is gone after a failed bundle %s", step)
			}
		})
	}
}

func TestC2558_006_FailedBundleStepOnASalvageKeepsTheTreeAndTheBranch(t *testing.T) {
	for _, step := range []string{"create", "verify"} {
		t.Run(step, func(t *testing.T) {
			s := newSweepRepo(t, true)
			tree, _ := s.addLaneTree("cycle-fail-2558")
			gittest.InstallFault(t, gittest.GitFault{Verb: "bundle", Args: []string{step}, Code: 1})

			err := s.salvageTree(tree, "cycle-fail-2558")

			if err == nil || !errors.Is(err, gc.ErrTreeKept) {
				t.Fatalf("salvage err = %v, want a refusal that names the kept tree", err)
			}
			if _, statErr := os.Stat(tree); statErr != nil {
				t.Errorf("the tree is gone after a failed bundle %s: %v", step, statErr)
			}
			if !gittest.BranchExists(t, s.repo.Dir, "cycle-fail-2558") {
				t.Errorf("the branch is gone after a failed bundle %s", step)
			}
			if got, readErr := os.ReadFile(filepath.Join(tree, "wip.txt")); readErr != nil || string(got) != "uncommitted\n" {
				t.Errorf("the uncommitted file = %q, %v; want it untouched in place", got, readErr)
			}
		})
	}
}

func TestC2558_007_WithoutAnOriginMainRefTheBundleStillHoldsTheBranchAndVerifies(t *testing.T) {
	s := newSweepRepo(t, false)
	tip := s.commitOn("lane/noorigin", "noorigin.txt")

	if err := s.sweepBranch("lane/noorigin", tip); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	leaves := s.leaves()
	if len(leaves) != 1 {
		t.Fatalf("leaves = %v, want one", leaves)
	}
	bundle := filepath.Join(leaves[0], bundleFileName)
	s.repo.Git("bundle", "verify", bundle)
	if heads := s.repo.Git("bundle", "list-heads", bundle); !strings.Contains(heads, tip+" refs/heads/lane/noorigin") {
		t.Errorf("bundle heads %q do not hold the branch at %s", heads, tip)
	}
}
