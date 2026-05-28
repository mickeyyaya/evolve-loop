// `evolve worktree` manages per-cycle git worktrees provisioned under
// EVOLVE_WORKTREE_BASE (default .evolve/worktrees/). This is the v1 port
// of the worktree-management surface from scripts/lifecycle/run-cycle.sh.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

// absWorktreeRoot absolutizes a worktree subcommand's --project-root (default
// ".") so the recorded worktree path and base dir are cwd-independent — the
// same canonicalization every entrypoint applies (cycle-119 path-divergence).
func absWorktreeRoot(projectRoot string, stderr io.Writer) string {
	return paths.AbsoluteRoot("--project-root", projectRoot, func(m string) {
		fmt.Fprintf(stderr, "evolve worktree: WARN: %s\n", m)
	})
}

// runWorktree implements `evolve worktree <subcommand>`. Subcommands:
// create | list | cleanup.
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
	)
	fs.IntVar(&cycle, "cycle", 0, "cycle number (required)")
	fs.StringVar(&projectRoot, "project-root", ".", "absolute path to project root")
	fs.StringVar(&base, "base", "", "worktree base dir (default .evolve/worktrees)")
	if err := fs.Parse(args); err != nil {
		return 10
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
	wt := filepath.Join(base, fmt.Sprintf("cycle-%d", cycle))
	cmd := exec.Command("git", "-C", projectRoot, "worktree", "add", "--detach", wt, "HEAD")
	var ebuf bytes.Buffer
	cmd.Stderr = &ebuf
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(stderr, "evolve worktree create: git: %v\n%s", err, ebuf.String())
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
	cmd := exec.Command("git", "-C", projectRoot, "worktree", "list")
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
	)
	fs.StringVar(&projectRoot, "project-root", ".", "absolute path to project root")
	fs.StringVar(&base, "base", "", "worktree base dir (default .evolve/worktrees)")
	fs.IntVar(&cycle, "cycle", 0, "cycle number to remove (0 = prune all stale)")
	if err := fs.Parse(args); err != nil {
		return 10
	}
	projectRoot = absWorktreeRoot(projectRoot, stderr)
	if base == "" {
		base = filepath.Join(projectRoot, ".evolve", "worktrees")
	}
	if cycle > 0 {
		wt := filepath.Join(base, fmt.Sprintf("cycle-%d", cycle))
		cmd := exec.Command("git", "-C", projectRoot, "worktree", "remove", "--force", wt)
		var ebuf bytes.Buffer
		cmd.Stderr = &ebuf
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(stderr, "evolve worktree cleanup: %v\n%s", err, ebuf.String())
			return 1
		}
		// Also remove leftover dir if git left an empty stub.
		if err := os.RemoveAll(wt); err != nil && !errIsNotExist(err) {
			fmt.Fprintf(stderr, "evolve worktree cleanup: rm %s: %v\n", wt, err)
		}
		fmt.Fprintln(stdout, wt)
		return 0
	}
	cmd := exec.Command("git", "-C", projectRoot, "worktree", "prune", "-v")
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(stderr, "evolve worktree cleanup: prune: %v\n", err)
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
