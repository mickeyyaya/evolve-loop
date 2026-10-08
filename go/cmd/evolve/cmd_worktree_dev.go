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
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
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

func cleanupDevFromFlags(projectRoot string, req devCleanupRequest, stdout, stderr io.Writer) int {
	opts := devCleanupOptions{mergedPRHeads: ghMergedPRHeads, dryRun: req.dryRun}
	if req.all {
		return runWorktreeCleanupDevAll(projectRoot, opts, stdout, stderr)
	}
	if !validDevTask(req.task) {
		fmt.Fprintf(stderr, "evolve worktree cleanup: --dev %q is not a task name\n", req.task)
		return 10
	}
	return runWorktreeCleanupDev(projectRoot, req.task, opts, stdout, stderr)
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

type mergedPRHeadsFunc func(ctx context.Context, dir, branch string) (string, error)

func ghMergedPRHeads(ctx context.Context, dir, branch string) (string, error) {
	gh, err := exec.LookPath("gh")
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, devGHTimeout)
	defer cancel()
	cmd := sysexec.Command(ctx, gh, "pr", "list", "--head", branch, "--state", "merged", "--json", "headRefOid", "--jq", ".[].headRefOid")
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
