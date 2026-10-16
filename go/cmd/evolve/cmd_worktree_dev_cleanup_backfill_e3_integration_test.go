//go:build integration

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

const e3Injected = "gittest: injected fault on git "

func e3CleanTree(t *testing.T) (landedHub, string) {
	t.Helper()
	h := newLandedHub(t)
	return h, h.dev(t, "t1", "b1")
}

func e3DirtyLandedTree(t *testing.T) (landedHub, string) {
	t.Helper()
	h, dir := e3CleanTree(t)
	changed := bigFile(map[int]string{5: "line 5 landed"})
	h.land(t, "big.txt", changed, "origin lands it")
	h.fetch(t)
	writeDevFile(t, dir, "big.txt", changed)
	return h, dir
}

func TestWorktreeDevCleanup_EveryGitFaultFailsTheTreeAndKeepsIt(t *testing.T) {
	notDir := filepath.Join(t.TempDir(), "plain-file")
	if err := os.WriteFile(notDir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name  string
		tree  func(t *testing.T) (landedHub, string)
		fault gittest.GitFault
		want  string
	}{
		{"worktree list", e3CleanTree, gittest.GitFault{Verb: "worktree", Args: []string{"list"}, Code: 128}, e3Injected},
		{"admin dir lookup", e3CleanTree, gittest.GitFault{Verb: "rev-parse", Args: []string{"--absolute-git-dir"}, Code: 128}, e3Injected},
		{"unreadable admin dir", e3CleanTree, gittest.GitFault{Verb: "rev-parse", Args: []string{"--absolute-git-dir"}, Stdout: notDir + "/admin"}, "quiet check: "},
		{"symbolic-ref", e3CleanTree, gittest.GitFault{Verb: "symbolic-ref", Code: 128}, "git symbolic-ref rc=128"},
		{"head", e3CleanTree, gittest.GitFault{Verb: "rev-parse", Args: []string{"HEAD"}, Code: 128}, e3Injected},
		{"status", e3CleanTree, gittest.GitFault{Verb: "status", Code: 128}, "git status -z rc=128"},
		{"ancestry", e3CleanTree, gittest.GitFault{Verb: "merge-base", Args: []string{"--is-ancestor"}, Code: 128}, "git merge-base --is-ancestor rc=128"},
		{"merge-base of a dirty tree", e3DirtyLandedTree, gittest.GitFault{Verb: "merge-base", Code: 128}, e3Injected},
		{"binary diff of a dirty tree", e3DirtyLandedTree, gittest.GitFault{Verb: "diff", Args: []string{"--binary"}, Code: 128}, e3Injected},
		{"raw diff of a dirty tree", e3DirtyLandedTree, gittest.GitFault{Verb: "diff", Args: []string{"--raw"}, Code: 128}, e3Injected},
		{"recheck before removal", e3DirtyLandedTree, gittest.GitFault{Verb: "diff", Args: []string{"--binary"}, Skip: 1, Code: 128}, e3Injected},
		{"worktree remove", e3CleanTree, gittest.GitFault{Verb: "worktree", Args: []string{"remove"}, Code: 128}, "git worktree remove rc=128"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, dir := tc.tree(t)
			gittest.InstallFault(t, tc.fault)

			out, code := cleanupOne(t, h, "t1", devCleanupOptions{})

			if code != devFailed || !strings.Contains(out, tc.want) {
				t.Fatalf("code %d out %q, want %d with %q", code, out, devFailed, tc.want)
			}
			if _, err := os.Stat(dir); err != nil {
				t.Errorf("a failed cleanup removed %s: %v", dir, err)
			}
		})
	}
}

func TestWorktreeDevCleanup_ABranchDeleteFaultAfterRemovalIsReported(t *testing.T) {
	h, dir := e3CleanTree(t)
	gittest.InstallFault(t, gittest.GitFault{Verb: "update-ref", Code: 128})

	out, code := cleanupOne(t, h, "t1", devCleanupOptions{})

	if code != devFailed || !strings.Contains(out, "tree removed but deleting branch b1 at ") {
		t.Fatalf("code %d out %q", code, out)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("the tree should be gone before the branch delete: %v", err)
	}
	if !gittest.BranchExists(t, h.store, "b1") {
		t.Error("branch b1 is gone although its delete failed")
	}
}

func TestWorktreeDevCleanup_AMergedPRAtTheHeadProvesASquashLanding(t *testing.T) {
	t.Parallel()
	h, dir := e3CleanTree(t)
	brCommit(t, dir, "feature.txt", "squashed elsewhere\n", "feature")
	head := brOut(t, dir, "rev-parse", "HEAD")
	var asked []string
	opts := devCleanupOptions{mergedPRHeads: func(_ context.Context, d, branch string) (string, error) {
		asked = append(asked, branch)
		return "deadbeef\n" + head + "\n", nil
	}}

	out, code := cleanupOne(t, h, "t1", opts)

	if code != 0 || !strings.Contains(out, "gh reports a merged PR for b1 whose head is "+head) {
		t.Fatalf("code %d out %q", code, out)
	}
	assertDevGone(t, h, dir, "b1", out)
	if len(asked) != 1 || asked[0] != "b1" {
		t.Errorf("gh was asked about %v, want b1 once", asked)
	}
}

func TestWorktreeDevCleanup_WithoutAProvableMergeTheReasonNamesWhatGHSaid(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		gh   func(context.Context, string, string) (string, error)
		want string
	}{
		{"gh missing", func(context.Context, string, string) (string, error) {
			return "", fmt.Errorf("lookpath: %w", exec.ErrNotFound)
		}, "gh is unavailable to prove a squash-merged PR"},
		{"gh failed", func(context.Context, string, string) (string, error) { return "", errors.New("HTTP 401") }, "gh pr list failed: HTTP 401"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, dir := e3CleanTree(t)
			brCommit(t, dir, "feature.txt", "not landed\n", "feature")

			out, code := cleanupOne(t, h, "t1", devCleanupOptions{mergedPRHeads: tc.gh})

			if code != devKept || !strings.Contains(out, "refused: branch b1 is not merged into origin/main") || !strings.Contains(out, tc.want) {
				t.Fatalf("code %d out %q", code, out)
			}
			assertDevKept(t, h, dir, "b1", out)
		})
	}
}

func TestWorktreeDevCleanup_RefusalsBeforeAnyJudgement(t *testing.T) {
	t.Parallel()
	t.Run("not a hub", func(t *testing.T) {
		r := bfCommittedRepo(t)
		for _, all := range []bool{false, true} {
			var out, errb bytes.Buffer
			var code int
			if all {
				code = runWorktreeCleanupDevAll(r.Dir, devCleanupOptions{mergedPRHeads: noMergedPR}, &out, &errb)
			} else {
				code = runWorktreeCleanupDev(r.Dir, "t1", devCleanupOptions{mergedPRHeads: noMergedPR}, &out, &errb)
			}
			if code != devKept || !strings.Contains(errb.String(), ": refused: ") || out.Len() != 0 {
				t.Errorf("all=%v: code %d stdout %q stderr %q", all, code, out.String(), errb.String())
			}
		}
	})
	t.Run("no such task", func(t *testing.T) {
		h := newLandedHub(t)
		out, code := cleanupOne(t, h, "ghost", devCleanupOptions{})
		if code != devKept || !strings.Contains(out, "refused: no dev worktree at ") || !strings.Contains(out, filepath.Join("dev", "ghost")) {
			t.Fatalf("code %d out %q", code, out)
		}
	})
	t.Run("dry run without a local origin/main", func(t *testing.T) {
		h, dir := e3CleanTree(t)
		brGit(t, h.store, "update-ref", "-d", devOriginMain)
		for _, all := range []bool{false, true} {
			var out string
			var code int
			if all {
				out, code = cleanupAll(t, h, devCleanupOptions{dryRun: true, now: futureNow()})
			} else {
				out, code = cleanupOne(t, h, "t1", devCleanupOptions{dryRun: true})
			}
			if code != devFailed || strings.Contains(out, "would remove") || strings.Contains(out, "dry run: judging") || strings.Contains(out, "kept,") {
				t.Errorf("all=%v: code %d out %q, want the rev-parse failure before any judgement", all, code, out)
			}
		}
		if _, err := os.Stat(dir); err != nil {
			t.Errorf("a dry run touched %s: %v", dir, err)
		}
	})
}

func TestWorktreeDevCleanupAll_FaultsAndStrayEntries(t *testing.T) {
	t.Run("dev is a file", func(t *testing.T) {
		h := newLandedHub(t)
		if err := os.WriteFile(filepath.Join(filepath.Dir(h.runtime), "dev"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		out, code := cleanupAll(t, h, devCleanupOptions{now: futureNow()})
		if code != devFailed || !strings.Contains(out, "evolve worktree cleanup --dev --all: ") || strings.Contains(out, "kept,") {
			t.Fatalf("code %d out %q", code, out)
		}
	})
	t.Run("a stray file is skipped", func(t *testing.T) {
		h, dir := e3CleanTree(t)
		writeDevFile(t, filepath.Dir(dir), "NOTES.md", "not a tree\n")
		out, code := cleanupAll(t, h, devCleanupOptions{now: futureNow()})
		if code != 0 || !strings.Contains(out, ": 1 removed, 0 kept, 0 failed") || strings.Contains(out, "NOTES.md") {
			t.Fatalf("code %d out %q", code, out)
		}
	})
	t.Run("a failing tree fails the run", func(t *testing.T) {
		h, dir := e3CleanTree(t)
		gittest.InstallFault(t, gittest.GitFault{Verb: "symbolic-ref", Code: 128})
		out, code := cleanupAll(t, h, devCleanupOptions{now: futureNow()})
		if code != devFailed || !strings.Contains(out, "failed dev/t1: git symbolic-ref rc=128") || !strings.Contains(out, ": 0 removed, 0 kept, 1 failed") {
			t.Fatalf("code %d out %q", code, out)
		}
		if _, err := os.Stat(dir); err != nil {
			t.Errorf("a failed tree was removed: %v", err)
		}
	})
	t.Run("an unreadable dirty path fails the quiet check", func(t *testing.T) {
		h, dir := e3CleanTree(t)
		writeDevFile(t, dir, "blk", "a file, not a dir\n")
		gittest.InstallFault(t, gittest.GitFault{Verb: "status", Stdout: "?? blk/x"})
		out, code := cleanupAll(t, h, devCleanupOptions{now: futureNow()})
		if code != devFailed || !strings.Contains(out, "failed dev/t1: quiet check: ") {
			t.Fatalf("code %d out %q", code, out)
		}
	})
}
