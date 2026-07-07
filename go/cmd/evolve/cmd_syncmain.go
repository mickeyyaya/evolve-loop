package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

// runSyncMain implements `evolve sync-main` — an operator/boundary command that
// reconciles a locally-diverged main with origin via a plain merge, so a
// recurring diverged-origin stall no longer needs manual `git` reconciliation
// (inbox ship-repair-merge-diverged-origin). It is the sanctioned successor to
// hand-running git after ship/repair refuses on a diverged origin (repair.go
// only ever preserves the local commit and points here — it never mutates the
// tree itself).
//
// Preconditions (ALL checked before any git mutation):
//   - no live run lease: an active cycle owns the tree; refuse (cycle-395 race).
//   - clean index: any uncommitted change blocks the sync (.evolve/** is
//     gitignored, so lease/cycle-state churn never counts as dirty).
//
// Behavior: fetch origin, then `git merge --no-edit origin/<branch>`. A clean
// divergence merges (real merge commit). A conflict aborts back to the exact
// pre-merge state (`git merge --abort`) and refuses. It NEVER rebases,
// force-pushes, or pushes — local history only ever moves forward via merge.
//
// Exit codes: 0 merged (or already up to date); 1 refused/error (tree unchanged
// on refusal).
func runSyncMain(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve sync-main", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var projectRoot string
	fs.StringVar(&projectRoot, "project-root", "", "project root (default: $EVOLVE_PROJECT_ROOT or cwd)")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	if projectRoot == "" {
		projectRoot = os.Getenv("EVOLVE_PROJECT_ROOT")
	}
	if projectRoot == "" {
		var err error
		if projectRoot, err = os.Getwd(); err != nil {
			fmt.Fprintf(stderr, "evolve sync-main: cwd: %v\n", err)
			return 1
		}
	}
	absRoot := paths.AbsoluteRoot("--project-root", projectRoot, func(m string) {
		fmt.Fprintf(stderr, "evolve sync-main: WARN: %s\n", m)
	})

	git := func(gitArgs ...string) (string, error) {
		cmd := exec.Command("git", append([]string{"-C", absRoot}, gitArgs...)...)
		out, err := cmd.CombinedOutput()
		return string(out), err
	}

	// Precondition 1: no live run lease. An active cycle owns the working tree;
	// merging under it is the cycle-395 clobber race. Mirrors cmd_loop.go's
	// live-owner probe (runlease.Read + OwnerLive + pidAlive).
	if ws := liveLeaseWorkspace(absRoot); ws != "" {
		if lease, ok, _ := runlease.Read(ws); ok && runlease.OwnerLive(lease, time.Now(), 0, pidAlive) {
			fmt.Fprintf(stderr, "evolve sync-main: refused — a run lease is live (pid %d, heartbeat fresh); another evolve loop owns this tree.\n", lease.OwnerPID)
			fmt.Fprintln(stderr, "evolve sync-main:   • let it finish, or `evolve loop --resume` to attach, then retry.")
			return 1
		}
	}

	// Precondition 2: clean index (.evolve/** is gitignored, so it never shows).
	porcelain, err := git("status", "--porcelain")
	if err != nil {
		fmt.Fprintf(stderr, "evolve sync-main: git status failed: %v\n%s", err, porcelain)
		return 1
	}
	if strings.TrimSpace(porcelain) != "" {
		fmt.Fprintf(stderr, "evolve sync-main: refused — working tree is dirty; commit or stash first:\n%s", porcelain)
		return 1
	}

	branch, err := git("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		fmt.Fprintf(stderr, "evolve sync-main: cannot resolve current branch: %v\n%s", err, branch)
		return 1
	}
	branch = strings.TrimSpace(branch)

	if out, err := git("fetch", "origin"); err != nil {
		fmt.Fprintf(stderr, "evolve sync-main: git fetch origin failed: %v\n%s", err, out)
		return 1
	}

	// Plain merge — no rebase, no force, no push. On conflict, abort cleanly so
	// the working tree and HEAD end up EXACTLY as they started.
	if out, err := git("merge", "--no-edit", "origin/"+branch); err != nil {
		if _, aerr := git("merge", "--abort"); aerr != nil {
			fmt.Fprintf(stderr, "evolve sync-main: merge conflicted AND abort failed: %v\n%s", aerr, out)
			return 1
		}
		fmt.Fprintf(stderr, "evolve sync-main: refused — merging origin/%s conflicts with local history (aborted cleanly, tree unchanged).\n", branch)
		fmt.Fprintln(stderr, "evolve sync-main:   • resolve manually, or re-audit the local commit on the new base.")
		return 1
	}

	fmt.Fprintf(stdout, "sync-main: reconciled local %s with origin/%s (merge only; nothing pushed).\n", branch, branch)
	return 0
}

// liveLeaseWorkspace returns the workspace_path recorded in
// <root>/.evolve/cycle-state.json, or "" when the marker is absent/unreadable
// or carries no workspace (matching gc.discover's minimal cycle_id/workspace
// schema — no dependency on the heavier core loader).
func liveLeaseWorkspace(root string) string {
	b, err := os.ReadFile(filepath.Join(root, ".evolve", "cycle-state.json"))
	if err != nil {
		return ""
	}
	var cs struct {
		WorkspacePath string `json:"workspace_path"`
	}
	if json.Unmarshal(b, &cs) != nil {
		return ""
	}
	return cs.WorkspacePath
}
