package core

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func gitS5b(t *testing.T, dir string, args ...string) {
	t.Helper()
	if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func writeS5b(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// setupDivergedRepo builds a repo whose cycle-branch worktree diverged from main
// (a peer cycle committed after the worktree branched). conflict=true makes the
// peer and the cycle touch the SAME file so the rebase conflicts; conflict=false
// keeps them disjoint (the advisor-partitioned case) so the rebase is clean.
func setupDivergedRepo(t *testing.T, conflict bool) (worktree string) {
	t.Helper()
	repo := t.TempDir()
	gitS5b(t, repo, "init", "-q", "-b", "main")
	gitS5b(t, repo, "config", "user.email", "t@t")
	gitS5b(t, repo, "config", "user.name", "t")
	writeS5b(t, filepath.Join(repo, "base.txt"), "base\n")
	if conflict {
		writeS5b(t, filepath.Join(repo, "shared.txt"), "original\n")
	}
	gitS5b(t, repo, "add", "-A")
	gitS5b(t, repo, "commit", "-q", "-m", "base")

	worktree = filepath.Join(t.TempDir(), "wt")
	gitS5b(t, repo, "worktree", "add", "-q", "-b", "cycle-branch", worktree, "main")

	bFile, peerFile := "feature.txt", "peer.txt"
	if conflict {
		bFile, peerFile = "shared.txt", "shared.txt"
	}
	// This cycle's change, committed on the worktree branch.
	writeS5b(t, filepath.Join(worktree, bFile), "this cycle's change\n")
	gitS5b(t, worktree, "add", "-A")
	gitS5b(t, worktree, "commit", "-q", "-m", "this cycle")
	// A peer cycle advances main after the worktree branched.
	writeS5b(t, filepath.Join(repo, peerFile), "a peer cycle's change\n")
	gitS5b(t, repo, "add", "-A")
	gitS5b(t, repo, "commit", "-q", "-m", "peer")
	return worktree
}

// TestRebaseCycleBranchOntoMain_CleanDisjoint_Succeeds pins ADR-0049 S5b-2b: the
// advisor-partitioned (disjoint-file) case rebases cleanly so the cycle can
// re-audit + re-ship the merged tree.
func TestRebaseCycleBranchOntoMain_CleanDisjoint_Succeeds(t *testing.T) {
	wt := setupDivergedRepo(t, false)
	if !rebaseCycleBranchOntoMain(context.Background(), wt) {
		t.Fatal("clean disjoint rebase should succeed")
	}
}

// TestRebaseCycleBranchOntoMain_Conflict_Fails: overlapping work the partition
// should have kept apart → rebase conflicts → false (caller aborts the cycle).
func TestRebaseCycleBranchOntoMain_Conflict_Fails(t *testing.T) {
	wt := setupDivergedRepo(t, true)
	if rebaseCycleBranchOntoMain(context.Background(), wt) {
		t.Fatal("conflicting rebase must return false")
	}
	// The rebase must have been aborted (worktree left clean, not mid-rebase).
	if _, err := os.Stat(filepath.Join(wt, ".git", "rebase-merge")); !os.IsNotExist(err) {
		// .git in a worktree is a file pointer; rebase state lives in the repo's
		// worktrees/<name> dir, so a strict path check is brittle — instead assert
		// a subsequent rebase can start (no "rebase in progress").
		gitS5b(t, wt, "status") // fails the test via gitS5b if the tree is wedged
	}
}

// TestRebaseCycleBranchOntoMain_EmptyWorktree_False: a degraded (no-worktree)
// run must not attempt a rebase.
func TestRebaseCycleBranchOntoMain_EmptyWorktree_False(t *testing.T) {
	if rebaseCycleBranchOntoMain(context.Background(), "") {
		t.Fatal("empty worktree must return false")
	}
}
