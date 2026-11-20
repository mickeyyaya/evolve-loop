package gc

import (
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

const thinBundleLimit = 1 << 20

func newThinBundleEnv(t *testing.T) *branchSweepEnv {
	t.Helper()
	e := newBranchSweepEnv(t)
	bulk := make([]byte, 3<<20)
	if _, err := rand.Read(bulk); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(e.repo.Dir, "bulk.bin"), bulk, 0o644); err != nil {
		t.Fatal(err)
	}
	e.repo.Git("add", "bulk.bin")
	e.repo.Git("commit", "-q", "-m", "bulk history")
	e.repo.Git("update-ref", "refs/remotes/origin/main", "refs/heads/main")
	e.opts.Exec = sysexec.DefaultRunner
	return e
}

func TestBundleBranch_ALeafOneCommitPastOriginMainHoldsOnlyThatCommit(t *testing.T) {
	e := newThinBundleEnv(t)
	tip := e.commitOn("lane/thin", "thin.txt")

	if err := e.opts.bundleBranch(WorktreeItem{Branch: "lane/thin", Head: tip}); err != nil {
		t.Fatalf("bundleBranch: %v", err)
	}

	found := e.bundles()
	if len(found) != 1 {
		t.Fatalf("bundles = %v, want one", found)
	}
	info, err := os.Stat(found[0])
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() >= thinBundleLimit {
		t.Errorf("bundle is %d bytes, want under %d: it holds the history origin/main already has", info.Size(), thinBundleLimit)
	}
	e.repo.Git("bundle", "verify", found[0])
}

func TestBundleBranch_ATipAlreadyInOriginMainWritesTheLeafHeadAndNoBundle(t *testing.T) {
	e := newThinBundleEnv(t)
	tip := e.land("lane/landed", "landed.txt")
	e.repo.Git("update-ref", "refs/remotes/origin/main", "refs/heads/main")

	if err := e.opts.bundleBranch(WorktreeItem{Branch: "lane/landed", Head: tip}); err != nil {
		t.Fatalf("bundleBranch: %v", err)
	}

	if found := e.bundles(); len(found) != 0 {
		t.Errorf("bundles = %v, want none for a tip that origin/main holds", found)
	}
	heads, err := filepath.Glob(filepath.Join(e.opts.EvolveDir, "operator-salvage", "*", salvageHeadFile))
	if err != nil || len(heads) != 1 {
		t.Fatalf("head files = %v, %v; want one", heads, err)
	}
	if got := readFile(t, heads[0]); got != tip+" lane/landed\n" {
		t.Errorf("leaf head = %q, want the tip and branch", got)
	}
}
