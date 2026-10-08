package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/runscope"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

// worktreeGitRunner is the command seam evolve worktree create provisions
// through, mirroring core's gitRunner and swarm's newGit precedents.
var worktreeGitRunner sysexec.RunFunc = sysexec.DefaultRunner

// worktreeAddRetry shares gitexec's single retry contract; Retryable is the
// same classifier core and swarm pass, so a non-repository fails immediately
// instead of waiting out a retry ladder that cannot help.
var worktreeAddRetry = gitexec.WorktreeAddRetry{Retryable: gitexec.RetryableWorktreeAddFailure}

// absWorktreeRoot absolutizes a worktree subcommand's --project-root so the
// recorded worktree path and base dir are cwd-independent.
func absWorktreeRoot(projectRoot string, stderr io.Writer) string {
	return paths.AbsoluteRoot("--project-root", projectRoot, func(m string) {
		fmt.Fprintf(stderr, "evolve worktree: WARN: %s\n", m)
	})
}

func runWorktree(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "evolve worktree: missing subcommand (create|list|cleanup)")
		return 10
	}
	switch args[0] {
	case "create":
		return runWorktreeCreate(args[1:], stdout, stderr)
	case "list":
		return runWorktreeList(args[1:], stdout, stderr)
	case "cleanup":
		return runWorktreeCleanup(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "evolve worktree: unknown subcommand %q\n", args[0])
		return 10
	}
}

func runWorktreeCreate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve worktree create", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		cycle       int
		projectRoot string
		base        string
		lane        string
		devTask     string
		devBranch   string
	)
	fs.IntVar(&cycle, "cycle", 0, "cycle number (required unless --dev)")
	fs.StringVar(&projectRoot, "project-root", ".", "absolute path to project root")
	fs.StringVar(&base, "base", "", "worktree base dir (default .evolve/worktrees)")
	fs.StringVar(&lane, "lane", "", "lane override for the worktree name (default: hash of project root, or EVOLVE_LANE)")
	fs.StringVar(&devTask, "dev", "", "create the hub dev worktree <hub>/dev/<task> on --branch at the fetched origin/main")
	fs.StringVar(&devBranch, "branch", "", "new branch for --dev")
	if err := fs.Parse(args); err != nil {
		return 10
	}
	if devTask != "" || devBranch != "" {
		return createDevFromFlags(absWorktreeRoot(projectRoot, stderr), devTask, devBranch, cycle, stdout, stderr)
	}
	if cycle <= 0 {
		fmt.Fprintln(stderr, "evolve worktree create: --cycle is required (>0)")
		return 10
	}
	projectRoot = absWorktreeRoot(projectRoot, stderr)
	if base == "" {
		base = filepath.Join(projectRoot, ".evolve", "worktrees")
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		fmt.Fprintf(stderr, "evolve worktree create: mkdir: %v\n", err)
		return 1
	}
	// runscope gives the SAME lane-namespaced path core.gitWorktree provisions,
	// so concurrent worktrees never collide.
	wt := runscope.New(runscope.ResolveLane(lane, projectRoot, os.Getenv), "", cycle).WorktreeDir(base)
	// The shared gitexec retry surfaces git's own exit code and stderr, so an
	// operator can tell lane contention from a real fault (a raw err only ever
	// reports "exit status 255").
	_, gitErr, code, err := gitexec.Git{Dir: projectRoot, Exec: worktreeGitRunner}.
		AddWorktreeWithRetry(context.Background(), worktreeAddRetry, "--detach", wt, "HEAD")
	if err != nil || code != 0 {
		fmt.Fprintf(stderr, "evolve worktree create: git: worktree add --detach %s HEAD: rc=%d err=%v\n%s", wt, code, err, gitErr)
		return 1
	}
	fmt.Fprintln(stdout, wt)
	return 0
}

func runWorktreeList(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve worktree list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var projectRoot string
	fs.StringVar(&projectRoot, "project-root", ".", "absolute path to project root")
	if err := fs.Parse(args); err != nil {
		return 10
	}
	projectRoot = absWorktreeRoot(projectRoot, stderr)
	cmd := sysexec.Command(context.Background(), "git", "-C", projectRoot, "worktree", "list")
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(stderr, "evolve worktree list: %v\n", err)
		return 1
	}
	return 0
}

func runWorktreeCleanup(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve worktree cleanup", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		projectRoot string
		base        string
		cycle       int
		lane        string
		dev         devCleanupRequest
		stale       bool
		apply       bool
	)
	fs.StringVar(&projectRoot, "project-root", ".", "absolute path to project root")
	fs.StringVar(&base, "base", "", "worktree base dir (default .evolve/worktrees)")
	fs.IntVar(&cycle, "cycle", 0, "cycle number to remove (0 = prune all stale)")
	fs.StringVar(&lane, "lane", "", "lane override; must match the lane used at create (default: hash of project root, or EVOLVE_LANE)")
	fs.StringVar(&dev.task, "dev", "", "remove the hub dev worktree <hub>/dev/<task> and its branch once its head is merged or every change in it is already in origin/main (a named task skips --all's quiet period)")
	fs.BoolVar(&dev.all, "all", false, "with a bare --dev: clean up every dev/<task> a proof shows landed and unchanged for gc.worktrees.dev_quiet_minutes (default "+gcpolicy.WorktreesPolicy{}.DevQuietPeriod().String()+"), naming the reason each other one is kept")
	fs.BoolVar(&dev.dryRun, "dry-run", false, "with --dev: print what would be removed and why each tree is kept; removes nothing and does not fetch")
	fs.BoolVar(&stale, "stale", false, "list sealed cycle worktrees no continuation binding or fresh run lease holds (dry-run unless --apply)")
	fs.BoolVar(&apply, "apply", false, "with --stale: remove the listed worktrees and their merged branches")
	args, bareDev := bareDevFlag(args)
	if err := fs.Parse(args); err != nil {
		return 10
	}
	if msg := dev.usage(bareDev); msg != "" {
		fmt.Fprintln(stderr, "evolve worktree cleanup: "+msg)
		return 10
	}
	if modes := btoi(dev.selected()) + btoi(stale) + btoi(cycle > 0); modes > 1 || (apply && !stale) {
		fmt.Fprintln(stderr, "evolve worktree cleanup: --dev, --stale and --cycle are exclusive; --apply needs --stale")
		return 10
	}
	projectRoot = absWorktreeRoot(projectRoot, stderr)
	if dev.selected() {
		return cleanupDevFromFlags(projectRoot, dev, stdout, stderr)
	}
	if base == "" {
		base = filepath.Join(projectRoot, ".evolve", "worktrees")
	}
	switch {
	case stale:
		return runWorktreeCleanupStale(projectRoot, base, apply, stdout, stderr)
	case cycle > 0:
		return removeCycleWorktree(projectRoot, base, lane, cycle, stdout, stderr)
	}
	return pruneWorktrees(projectRoot, stdout, stderr)
}

func removeCycleWorktree(projectRoot, base, lane string, cycle int, stdout, stderr io.Writer) int {
	wt := runscope.New(runscope.ResolveLane(lane, projectRoot, os.Getenv), "", cycle).WorktreeDir(base)
	cmd := sysexec.Command(context.Background(), "git", "-C", projectRoot, "worktree", "remove", "--force", wt)
	var ebuf bytes.Buffer
	cmd.Stderr = &ebuf
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(stderr, "evolve worktree cleanup: %v\n%s", err, ebuf.String())
		return 1
	}
	if err := os.RemoveAll(wt); err != nil && !errIsNotExist(err) {
		fmt.Fprintf(stderr, "evolve worktree cleanup: rm %s: %v\n", wt, err)
	}
	fmt.Fprintln(stdout, wt)
	return 0
}

func pruneWorktrees(projectRoot string, stdout, stderr io.Writer) int {
	cmd := sysexec.Command(context.Background(), "git", "-C", projectRoot, "worktree", "prune", "-v")
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(stderr, "evolve worktree cleanup: prune: %v\n", err)
		return 1
	}
	return 0
}

func bareDevFlag(args []string) ([]string, bool) {
	out := slices.Clone(args)
	bare := false
	for i, a := range out {
		if (a == "-dev" || a == "--dev") && (i+1 == len(out) || strings.HasPrefix(out[i+1], "-")) {
			out[i] = "--dev="
			bare = true
		}
	}
	return out, bare
}

type devCleanupRequest struct {
	task        string
	all, dryRun bool
}

func (r devCleanupRequest) selected() bool { return r.task != "" || r.all }

func (r devCleanupRequest) usage(bareDev bool) string {
	switch {
	case r.all && r.task != "":
		return "--dev <task> and --all are exclusive"
	case r.all && !bareDev:
		return "--all needs --dev (evolve worktree cleanup --dev --all)"
	case bareDev && !r.all:
		return "--dev needs a task name, or --all"
	case r.dryRun && !r.selected():
		return "--dry-run needs --dev"
	}
	return ""
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

func errIsNotExist(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "no such file or directory") || os.IsNotExist(err)
}
