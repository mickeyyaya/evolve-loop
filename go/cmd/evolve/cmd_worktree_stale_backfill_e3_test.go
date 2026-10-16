package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func e3StaleRepo(t *testing.T, cycles ...int) (*gittest.Repo, string) {
	t.Helper()
	r := gittest.Fixture(t)
	r.Git("commit", "-q", "--allow-empty", "-m", "base")
	base := filepath.Join(r.Dir, ".evolve", "worktrees")
	for _, n := range cycles {
		r.Git("worktree", "add", "-q", "-b", staleLeaf(n), filepath.Join(base, staleLeaf(n)), "main")
		staleSeal(t, r.Dir, n)
	}
	return r, base
}

func e3RunStale(root string, extra ...string) (string, string, int) {
	var out, errb bytes.Buffer
	code := runWorktree(append([]string{"cleanup", "--stale", "--project-root", root}, extra...), nil, &out, &errb)
	return out.String(), errb.String(), code
}

func TestWorktreeCleanupStale_FailuresToPlanExitOne(t *testing.T) {
	t.Run("not a repository", func(t *testing.T) {
		root := e3StaleNonRepo(t)
		out, errb, code := e3RunStale(root)
		if code != 1 || out != "" || !strings.HasPrefix(errb, "evolve worktree cleanup --stale: list worktrees: ") {
			t.Fatalf("code %d stdout %q stderr %q", code, out, errb)
		}
	})
	t.Run("malformed continuation registry", func(t *testing.T) {
		r, _ := e3StaleRepo(t, 1)
		writeFileForPublishTest(t, continuation.RegistryPath(r.Dir), "{not json")
		out, errb, code := e3RunStale(r.Dir)
		if code != 1 || out != "" || !strings.Contains(errb, "continuation: parse registry: ") {
			t.Fatalf("code %d stdout %q stderr %q", code, out, errb)
		}
	})
	t.Run("a lane git cannot read", func(t *testing.T) {
		r, base := e3StaleRepo(t, 1)
		writeFileForPublishTest(t, filepath.Join(base, staleLeaf(1), ".git"), "gitdir: "+filepath.Join(t.TempDir(), "gone")+"\n")
		out, errb, code := e3RunStale(r.Dir)
		if code != 1 || out != "" || !strings.HasPrefix(errb, "evolve worktree cleanup --stale: "+staleLeaf(1)+": ") {
			t.Fatalf("code %d stdout %q stderr %q", code, out, errb)
		}
	})
}

func e3StaleNonRepo(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))
	return dir
}

func TestWorktreeCleanupStale_NoCycleWorktreesIsASuccess(t *testing.T) {
	t.Parallel()
	r, base := e3StaleRepo(t)
	r.Git("worktree", "add", "-q", "-b", "side", filepath.Join(base, "not-a-cycle"), "main")

	out, errb, code := e3RunStale(r.Dir)

	if code != 0 || errb != "" || out != "no cycle worktrees under "+base+"\n" {
		t.Fatalf("code %d stdout %q stderr %q", code, out, errb)
	}
}

func TestWorktreeCleanupStale_KeepReasonsForAMissingDirAndABranchBinding(t *testing.T) {
	t.Parallel()
	r, base := e3StaleRepo(t, 1, 2)
	if err := os.RemoveAll(filepath.Join(base, staleLeaf(1))); err != nil {
		t.Fatal(err)
	}
	if err := continuation.WriteRegistryEntry(r.Dir, "scope-b", continuation.Continuation{Branch: staleLeaf(2), Cycle: 2}); err != nil {
		t.Fatal(err)
	}

	out, errb, code := e3RunStale(r.Dir, "--apply")

	if code != 0 || errb != "" {
		t.Fatalf("code %d stderr %q", code, errb)
	}
	if !brLine(out, staleLeaf(1)+" ", "kept: worktree directory unreadable", "prunes a missing one") {
		t.Errorf("missing-dir lane not kept with its reason:\n%s", out)
	}
	if !brLine(out, staleLeaf(2)+" ", "kept: named by continuation binding scope-b") {
		t.Errorf("branch-bound lane not kept:\n%s", out)
	}
	if !gittest.BranchExists(t, r.Dir, staleLeaf(1)) || !gittest.BranchExists(t, r.Dir, staleLeaf(2)) {
		t.Error("a kept lane lost its branch")
	}
}

func TestWorktreeCleanupStale_ALockedLaneFailsTheApply(t *testing.T) {
	t.Parallel()
	r, base := e3StaleRepo(t, 1)
	r.Git("worktree", "lock", filepath.Join(base, staleLeaf(1)))

	out, errb, code := e3RunStale(r.Dir, "--apply")

	if code != 1 || strings.Contains(out, "removed") || !strings.HasPrefix(errb, "evolve worktree cleanup --stale: "+staleLeaf(1)+": git worktree remove rc=") {
		t.Fatalf("code %d stdout %q stderr %q", code, out, errb)
	}
	if _, err := os.Stat(filepath.Join(base, staleLeaf(1))); err != nil {
		t.Errorf("the locked lane is gone: %v", err)
	}
}

func TestWorktreeCleanupStale_AnUnmergedBranchIsLeftInPlace(t *testing.T) {
	t.Parallel()
	r, base := e3StaleRepo(t, 1)
	lane := filepath.Join(base, staleLeaf(1))
	brCommit(t, lane, "lane.txt", "unmerged\n", "lane work")

	out, errb, code := e3RunStale(r.Dir, "--apply")

	if code != 0 || errb != "" || !brLine(out, staleLeaf(1)+" ", "removed", "left in place, not merged into HEAD") {
		t.Fatalf("code %d stdout %q stderr %q", code, out, errb)
	}
	if _, err := os.Stat(lane); !os.IsNotExist(err) {
		t.Errorf("lane dir survived: %v", err)
	}
	if !gittest.BranchExists(t, r.Dir, staleLeaf(1)) {
		t.Error("an unmerged branch was deleted")
	}
}

func TestRemoveStaleLane_ALeaseThatAppearedBeforeRemovalKeepsTheLane(t *testing.T) {
	t.Parallel()
	r, base := e3StaleRepo(t, 3)
	staleLease(t, r.Dir, 3)
	l := staleLane{leaf: staleLeaf(3), path: filepath.Join(base, staleLeaf(3)), branch: staleLeaf(3), cycle: 3}
	var out, errb bytes.Buffer

	failed := removeStaleLane(context.Background(), r.Dir, l, &out, &errb)

	if failed || errb.Len() != 0 || out.String() != staleLeaf(3)+" kept: run lease became fresh before removal (run unit) path="+l.path+"\n" {
		t.Fatalf("failed=%v stdout %q stderr %q", failed, out.String(), errb.String())
	}
	if _, err := os.Stat(l.path); err != nil {
		t.Errorf("a leased lane was removed: %v", err)
	}
}

func TestRemoveStaleLane_ALaneWithoutABranchReportsNone(t *testing.T) {
	t.Parallel()
	r, base := e3StaleRepo(t, 6)
	l := staleLane{leaf: staleLeaf(6), path: filepath.Join(base, staleLeaf(6)), cycle: 6}
	var out, errb bytes.Buffer

	failed := removeStaleLane(context.Background(), r.Dir, l, &out, &errb)

	if failed || out.String() != staleLeaf(6)+" removed path="+l.path+" branch=none\n" {
		t.Fatalf("failed=%v stdout %q stderr %q", failed, out.String(), errb.String())
	}
	if !gittest.BranchExists(t, r.Dir, staleLeaf(6)) {
		t.Error("a lane with no recorded branch must not delete one")
	}
}

func TestLaneBindingsBound(t *testing.T) {
	t.Parallel()
	b := laneBindings{byBranch: map[string]string{"lane-b": "scope-branch", "": "scope-empty"}, byLeaf: map[string]string{"cycle-x-1": "scope-leaf"}}
	cases := []struct {
		name      string
		l         staleLane
		wantScope string
		wantOK    bool
	}{
		{"branch match", staleLane{branch: "lane-b", leaf: "cycle-x-1"}, "scope-branch", true},
		{"leaf match", staleLane{branch: "other", leaf: "cycle-x-1"}, "scope-leaf", true},
		{"an empty branch never matches an empty binding", staleLane{leaf: "cycle-y-2"}, "", false},
		{"no match", staleLane{branch: "other", leaf: "cycle-z-3"}, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			scope, ok := b.bound(tc.l)
			if scope != tc.wantScope || ok != tc.wantOK {
				t.Errorf("bound = %q,%v want %q,%v", scope, ok, tc.wantScope, tc.wantOK)
			}
		})
	}
}

func TestLiveLaneRef(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	staleLease(t, root, 12)
	live := liveLaneRef(root, time.Now())
	cases := map[string]bool{"cycle-12": true, "cycle-13": false, "main": false, "": false, "cycle-": false, "cycle-feed01-12": true}
	for ref, want := range cases {
		if got := live(ref); got != want {
			t.Errorf("liveLaneRef(%q) = %v, want %v", ref, got, want)
		}
	}
}

func TestResolvedPath(t *testing.T) {
	t.Parallel()
	real, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		link:                            real,
		"/no/such/dir/../x/./y":         "/no/such/x/y",
		real + string(os.PathSeparator): real,
	}
	for in, want := range cases {
		if got := resolvedPath(in); got != want {
			t.Errorf("resolvedPath(%q) = %q, want %q", in, got, want)
		}
	}
}
