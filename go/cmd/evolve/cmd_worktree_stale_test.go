package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

func staleLeaf(n int) string { return fmt.Sprintf("cycle-feed01-%d", n) }

func staleSeal(t *testing.T, root string, n int) {
	t.Helper()
	dir := filepath.Join(root, "knowledge-base", "cycles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("cycle-%d.json", n)), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func staleLease(t *testing.T, root string, n int) {
	t.Helper()
	dir := filepath.Join(root, ".evolve", "runs", fmt.Sprintf("cycle-%d", n))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runlease.Write(dir, runlease.Lease{RunID: "unit", OwnerPID: os.Getpid()}, time.Now()); err != nil {
		t.Fatal(err)
	}
}

func staleFixture(t *testing.T) (*gittest.Repo, map[int]string) {
	t.Helper()
	r := gittest.Fixture(t)
	r.Git("commit", "-q", "--allow-empty", "-m", "base")
	want := map[int]string{1: "would-remove", 2: "dirty", 3: "not sealed", 4: "lease", 5: "continuation binding"}
	for n := range want {
		r.Git("worktree", "add", "-q", "-b", staleLeaf(n), filepath.Join(r.Dir, ".evolve", "worktrees", staleLeaf(n)), "main")
	}
	r.Git("commit", "-q", "--allow-empty", "-m", "advance")
	for _, n := range []int{1, 2, 4, 5} {
		staleSeal(t, r.Dir, n)
	}
	if err := os.WriteFile(filepath.Join(r.Dir, ".evolve", "worktrees", staleLeaf(2), "scratch.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	staleLease(t, r.Dir, 4)
	binding := continuation.Continuation{Worktree: filepath.Join("/elsewhere", staleLeaf(5)), Cycle: 5}
	if err := continuation.WriteRegistryEntry(r.Dir, "scope-5", binding); err != nil {
		t.Fatal(err)
	}
	return r, want
}

func runStale(t *testing.T, root string, extra ...string) string {
	t.Helper()
	var out, errb bytes.Buffer
	if code := runWorktree(append([]string{"cleanup", "--stale", "--project-root", root}, extra...), nil, &out, &errb); code != 0 {
		t.Fatalf("cleanup --stale %v exit=%d\n%s\n%s", extra, code, out.String(), errb.String())
	}
	return out.String()
}

func TestWorktreeCleanupStale_DryRunListsEveryReasonApplyRemovesOnlyTheFreeLane(t *testing.T) {
	t.Parallel()
	r, want := staleFixture(t)
	out := runStale(t, r.Dir)
	for n, needle := range want {
		if !brLine(out, staleLeaf(n)+" ", needle) {
			t.Errorf("dry-run line for %s must say %q\n%s", staleLeaf(n), needle, out)
		}
		if !brBranchExists(t, r.Dir, staleLeaf(n)) {
			t.Errorf("dry-run deleted branch %s", staleLeaf(n))
		}
	}
	out = runStale(t, r.Dir, "--apply")
	if !brLine(out, staleLeaf(1)+" ", "removed", "deleted") {
		t.Errorf("--apply must remove %s and delete its branch\n%s", staleLeaf(1), out)
	}
	for n := range want {
		_, err := os.Stat(filepath.Join(r.Dir, ".evolve", "worktrees", staleLeaf(n)))
		if gone := os.IsNotExist(err); gone != (n == 1) {
			t.Errorf("%s removed=%t, want %t\n%s", staleLeaf(n), gone, n == 1, out)
		}
		if brBranchExists(t, r.Dir, staleLeaf(n)) != (n != 1) {
			t.Errorf("%s branch presence wrong after --apply\n%s", staleLeaf(n), out)
		}
	}
}

func TestWorktreeCleanup_ExclusiveModesAreUsageErrors(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{
		{"cleanup", "--stale", "--cycle", "3"},
		{"cleanup", "--stale", "--dev", "t"},
		{"cleanup", "--apply"},
		{"cleanup", "--dev", "a/b"},
		{"create", "--dev", "t"},
		{"create", "--branch", "b"},
		{"create", "--dev", "t", "--branch", "b", "--cycle", "2"},
	} {
		var out, errb bytes.Buffer
		if code := runWorktree(append(args, "--project-root", t.TempDir()), nil, &out, &errb); code != 10 {
			t.Errorf("evolve worktree %s: exit=%d, want 10\n%s", strings.Join(args, " "), code, errb.String())
		}
	}
}

func TestBranchesAuditAndPrune_LiveLaneBranchIsLiveAndKept(t *testing.T) {
	t.Parallel()
	dir := brFixture(t)
	staleLease(t, dir, 100)
	stdout, code := brRun(t, dir, "audit")
	if code != 0 || !brLine(stdout, "cycle-100 ", "live", "superseded=false") {
		t.Errorf("audit must report the leased cycle-100 as live, superseded=false (exit=%d)\n%s", code, stdout)
	}
	stdout, code = brRun(t, dir, "prune", "--dry-run=false")
	if code != 0 || !brLine(stdout, "cycle-100 ", "live", "kept") {
		t.Errorf("prune must keep the leased cycle-100 as live (exit=%d)\n%s", code, stdout)
	}
	if !brBranchExists(t, dir, "cycle-100") {
		t.Error("prune deleted a live lane's branch")
	}
}
