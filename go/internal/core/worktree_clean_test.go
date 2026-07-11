package core

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Regression tests for the cycle-653 dirty-worktree-reuse incident: a reused
// per-cycle worktree inherited a prior failed attempt's uncommitted orphan RED
// test, ship bound the whole tree, and a would-PASS cycle failed. The
// cycle-584 lesson's prescribed gate (clean-HEAD provisioning with quarantine,
// never silent deletion) is ensureCleanWorktree, called on the
// gitWorktree.Create reuse branch.

func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	run("init", "-q")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("clean content at HEAD\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "tracked.txt")
	run("commit", "-q", "-m", "base")
	return dir
}

func TestEnsureCleanWorktree_QuarantinesInheritedOrphanRedTest(t *testing.T) {
	wt := initTestRepo(t)
	root := t.TempDir()

	// Cycle-653 reproduction: the reuse candidate carries a prior attempt's
	// uncommitted orphan RED test (untracked) AND a tracked-file modification.
	orphan := filepath.Join(wt, "go", "internal", "echo", "veto_red_test.go")
	if err := os.MkdirAll(filepath.Dir(orphan), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(orphan, []byte("package echo // orphan RED test from prior attempt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wt, "tracked.txt"), []byte("dirty modification from prior attempt\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	moved, err := ensureCleanWorktree(context.Background(), wt, root, 653)
	if err != nil {
		t.Fatalf("ensureCleanWorktree: %v", err)
	}
	if len(moved) != 2 {
		t.Fatalf("quarantined paths = %v, want 2 (orphan test + tracked modification)", moved)
	}

	// Dirt is PRESERVED on disk under the quarantine dir — never deleted.
	qdir := quarantineDir(root, 653)
	qOrphan := filepath.Join(qdir, "go", "internal", "echo", "veto_red_test.go")
	b, err := os.ReadFile(qOrphan)
	if err != nil {
		t.Fatalf("quarantined orphan RED test missing: %v", err)
	}
	if !strings.Contains(string(b), "orphan RED test") {
		t.Fatalf("quarantined orphan content lost: %q", b)
	}
	if _, err := os.ReadFile(filepath.Join(qdir, "tracked.txt")); err != nil {
		t.Fatalf("quarantined tracked modification missing: %v", err)
	}

	// Worktree is clean at HEAD: orphan gone, tracked content restored.
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Fatalf("orphan RED test still present in worktree (err=%v)", err)
	}
	got, err := os.ReadFile(filepath.Join(wt, "tracked.txt"))
	if err != nil || string(got) != "clean content at HEAD\n" {
		t.Fatalf("tracked.txt not restored to HEAD: %q err=%v", got, err)
	}
	if dirty := porcelainDirtySet(context.Background(), wt); len(dirty) != 0 {
		t.Fatalf("worktree still dirty after clean-provision: %v", dirty)
	}
}

func TestEnsureCleanWorktree_CleanWorktreeIsNoOp(t *testing.T) {
	wt := initTestRepo(t)
	root := t.TempDir()

	moved, err := ensureCleanWorktree(context.Background(), wt, root, 682)
	if err != nil {
		t.Fatalf("ensureCleanWorktree on clean worktree: %v", err)
	}
	if len(moved) != 0 {
		t.Fatalf("clean worktree quarantined %v, want nothing (false positive)", moved)
	}
	if _, err := os.Stat(quarantineDir(root, 682)); !os.IsNotExist(err) {
		t.Fatalf("quarantine dir created for a clean worktree (err=%v)", err)
	}
}

func TestEnsureCleanWorktree_RepeatQuarantineNeverOverwritesSalvage(t *testing.T) {
	wt := initTestRepo(t)
	root := t.TempDir()

	write := func(content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(wt, "junk.txt"), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("first attempt\n")
	if _, err := ensureCleanWorktree(context.Background(), wt, root, 7); err != nil {
		t.Fatal(err)
	}
	write("second attempt\n")
	if _, err := ensureCleanWorktree(context.Background(), wt, root, 7); err != nil {
		t.Fatal(err)
	}

	qdir := quarantineDir(root, 7)
	first, err := os.ReadFile(filepath.Join(qdir, "junk.txt"))
	if err != nil || string(first) != "first attempt\n" {
		t.Fatalf("earlier salvage overwritten: %q err=%v", first, err)
	}
	second, err := os.ReadFile(filepath.Join(qdir, "junk.txt.1"))
	if err != nil || string(second) != "second attempt\n" {
		t.Fatalf("second salvage not uniquely suffixed: %q err=%v", second, err)
	}
}
