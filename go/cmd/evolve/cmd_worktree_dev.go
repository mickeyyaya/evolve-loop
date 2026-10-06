package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/plane"
)

const (
	devOriginMain = plane.OriginMainRef
	devGHTimeout  = 30 * time.Second
)

type devHub struct{ root, store, dev string }

func resolveDevHub(projectRoot string) (devHub, error) {
	hub, err := plane.ResolveHub(projectRoot)
	if err != nil {
		return devHub{}, err
	}
	return devHub{root: hub.Root, store: hub.Store, dev: hub.DevDir()}, nil
}

func (h devHub) taskDir(task string) string { return filepath.Join(h.dev, task) }

func validDevTask(task string) bool {
	return task != "" && task != "." && task != ".." && !strings.ContainsAny(task, `/\`)
}

func fetchOriginMain(ctx context.Context, store gitexec.Git) error {
	_, gitErr, code, err := store.Capture(ctx, "fetch", "--quiet", "origin", "+refs/heads/main:"+devOriginMain)
	if err != nil || code != 0 {
		return fmt.Errorf("git fetch origin main: rc=%d err=%v: %s", code, err, strings.TrimSpace(gitErr))
	}
	return nil
}

func createDevFromFlags(projectRoot, task, branch string, cycle int, stdout, stderr io.Writer) int {
	if !validDevTask(task) || branch == "" || cycle != 0 {
		fmt.Fprintln(stderr, "evolve worktree create: --dev <task> needs --branch <name>, a task name without path separators, and no --cycle")
		return 10
	}
	return runWorktreeCreateDev(projectRoot, task, branch, stdout, stderr)
}

func cleanupDevFromFlags(projectRoot, task string, stdout, stderr io.Writer) int {
	if !validDevTask(task) {
		fmt.Fprintf(stderr, "evolve worktree cleanup: --dev %q is not a task name\n", task)
		return 10
	}
	return runWorktreeCleanupDev(projectRoot, task, ghMergedPRHeads, stdout, stderr)
}

func runWorktreeCreateDev(projectRoot, task, branch string, stdout, stderr io.Writer) int {
	const name = "evolve worktree create --dev"
	ctx := context.Background()
	hub, err := resolveDevHub(projectRoot)
	if err != nil {
		fmt.Fprintf(stderr, "%s: refused: %v\n", name, err)
		return 1
	}
	dir := hub.taskDir(task)
	if _, err := os.Lstat(dir); err == nil {
		fmt.Fprintf(stderr, "%s: refused: %s already exists\n", name, dir)
		return 1
	} else if !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(stderr, "%s: stat %s: %v\n", name, dir, err)
		return 2
	}
	store := gitexec.Default(hub.store)
	if _, _, code, err := store.Capture(ctx, "check-ref-format", "--branch", branch); err != nil || code != 0 {
		fmt.Fprintf(stderr, "%s: refused: %q is not a valid branch name\n", name, branch)
		return 1
	}
	switch _, gitErr, code, err := store.Capture(ctx, "show-ref", "--verify", "--quiet", "refs/heads/"+branch); {
	case err != nil || code > 1:
		fmt.Fprintf(stderr, "%s: git show-ref rc=%d err=%v: %s\n", name, code, err, strings.TrimSpace(gitErr))
		return 2
	case code == 0:
		fmt.Fprintf(stderr, "%s: refused: branch %s already exists\n", name, branch)
		return 1
	}
	if err := fetchOriginMain(ctx, store); err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", name, err)
		return 2
	}
	if _, gitErr, code, err := store.Capture(ctx, "worktree", "add", "--no-track", "-b", branch, dir, devOriginMain); err != nil || code != 0 {
		fmt.Fprintf(stderr, "%s: git worktree add rc=%d err=%v: %s\n", name, code, err, strings.TrimSpace(gitErr))
		return 2
	}
	fmt.Fprintln(stdout, dir)
	return 0
}

func runWorktreeCleanupDev(projectRoot, task string, mergedPRHeads mergedPRHeadsFunc, stdout, stderr io.Writer) int {
	const name = "evolve worktree cleanup --dev"
	ctx := context.Background()
	hub, err := resolveDevHub(projectRoot)
	if err != nil {
		fmt.Fprintf(stderr, "%s: refused: %v\n", name, err)
		return 1
	}
	dir := hub.taskDir(task)
	if _, err := os.Stat(dir); err != nil {
		fmt.Fprintf(stderr, "%s: refused: no dev worktree at %s: %v\n", name, dir, err)
		return 1
	}
	branch, head, code, err := cleanDevTreeHead(ctx, dir)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", name, err)
		return code
	}
	store := gitexec.Default(hub.store)
	if err := fetchOriginMain(ctx, store); err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", name, err)
		return 2
	}
	merged, how, err := devBranchMerged(ctx, store, mergedPRHeads, dir, branch, head)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", name, err)
		return 2
	}
	if !merged {
		fmt.Fprintf(stderr, "%s: refused: branch %s is not merged into origin/main (%s); tree and branch kept\n", name, branch, how)
		return 1
	}
	if _, gitErr, code, err := store.Capture(ctx, "worktree", "remove", dir); err != nil || code != 0 {
		fmt.Fprintf(stderr, "%s: git worktree remove rc=%d err=%v: %s\n", name, code, err, strings.TrimSpace(gitErr))
		return 2
	}
	if _, gitErr, code, err := store.Capture(ctx, "update-ref", "-d", "refs/heads/"+branch, head); err != nil || code != 0 {
		fmt.Fprintf(stderr, "%s: tree removed but deleting branch %s at %s failed rc=%d err=%v: %s\n", name, branch, head, code, err, strings.TrimSpace(gitErr))
		return 2
	}
	fmt.Fprintf(stdout, "removed %s and branch %s (%s)\n", dir, branch, how)
	return 0
}

func cleanDevTreeHead(ctx context.Context, dir string) (branch, head string, code int, err error) {
	tree := gitexec.Default(dir)
	dirty, err := tree.DirtyPaths(ctx)
	if err != nil {
		return "", "", 2, err
	}
	if len(dirty) > 0 {
		return "", "", 1, fmt.Errorf("refused: %s is dirty (%d uncommitted path(s): %s)", dir, len(dirty), strings.Join(dirty, ", "))
	}
	if branch, err = tree.Output(ctx, "symbolic-ref", "--quiet", "--short", "HEAD"); err != nil {
		return "", "", 1, fmt.Errorf("refused: %s is not on a branch: %w", dir, err)
	}
	if head, err = tree.HEAD(ctx); err != nil {
		return "", "", 2, err
	}
	return branch, head, 0, nil
}

func devBranchMerged(ctx context.Context, store gitexec.Git, mergedPRHeads mergedPRHeadsFunc, dir, branch, head string) (bool, string, error) {
	switch _, gitErr, code, err := store.Capture(ctx, "merge-base", "--is-ancestor", head, devOriginMain); {
	case err != nil || code > 1:
		return false, "", fmt.Errorf("git merge-base --is-ancestor rc=%d err=%v: %s", code, err, strings.TrimSpace(gitErr))
	case code == 0:
		return true, "head is in origin/main", nil
	}
	out, err := mergedPRHeads(ctx, dir, branch)
	switch {
	case errors.Is(err, exec.ErrNotFound):
		return false, "head not in origin/main and gh is unavailable to prove a squash-merged PR", nil
	case err != nil:
		return false, fmt.Sprintf("head not in origin/main and gh pr list failed: %v", err), nil
	}
	if mergedPRHeadIs(out, head) {
		return true, fmt.Sprintf("gh reports a merged PR for %s whose head is %s", branch, head), nil
	}
	return false, fmt.Sprintf("head %s not in origin/main and no merged PR for %s has it as its head", head, branch), nil
}

type mergedPRHeadsFunc func(ctx context.Context, dir, branch string) (string, error)

func ghMergedPRHeads(ctx context.Context, dir, branch string) (string, error) {
	gh, err := exec.LookPath("gh")
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, devGHTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, gh, "pr", "list", "--head", branch, "--state", "merged", "--json", "headRefOid", "--jq", ".[].headRefOid")
	cmd.Dir = dir
	out, err := cmd.Output()
	return string(out), err
}

func mergedPRHeadIs(headRefOids, head string) bool {
	for _, oid := range strings.Fields(headRefOids) {
		if oid == head {
			return true
		}
	}
	return false
}
