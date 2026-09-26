package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/swarm"
)

// runGC implements `evolve gc [--dry-run]`.
func runGC(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evolve gc", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dryRun := fs.Bool("dry-run", false, "preview only: list the orphan sessions and workspace (worktree/branch) items that WOULD be reaped, mutating nothing")
	// The back-quoted `dir` is the flag package's argument placeholder (it
	// renders as "-project-root dir"); no other back-quotes here, or the first
	// one would be consumed as the placeholder instead.
	projectRoot := fs.String("project-root", "", "repository root `dir` the workspace (worktree/branch) sweep is aimed at; default = current directory.\n\tNOTE the deliberate asymmetry: an explicit 'evolve gc' APPLIES the workspace sweep (an operator run is enforce),\n\twhile the in-loop hook's default mode stays 'shadow' (plan + publish only) — policy.json owns the loop's mode,\n\tthe operator owns their own invocation. Use --dry-run to preview.")
	if err := fs.Parse(args); err != nil {
		return 10
	}
	// Bound the sweep so a wedged tmux socket can't hang the command.
	ctx, cancel := context.WithTimeout(context.Background(), orphanGCTimeout)
	defer cancel()

	var rep swarm.OrphanReapReport
	if *dryRun {
		// A no-op killer turns the sweep into a preview: the report's Killed
		// list is exactly what a real run would reap.
		noop := func(_ context.Context, _ string) error { return nil }
		rep = swarm.ReapOrphanSessions(ctx, swarm.ExecListBridgeSessions, swarm.ExecPidAlive, noop)
		fmt.Fprintf(stdout, "evolve gc --dry-run: %d orphan session(s) would be reaped\n", len(rep.Killed))
		for _, s := range rep.Killed {
			fmt.Fprintf(stdout, "  WOULD-REAP %s\n", s)
		}
	} else {
		rep = swarm.ExecReapOrphans(ctx)
		fmt.Fprintf(stdout, "evolve gc: reaped %d orphan session(s)\n", len(rep.Killed))
		for _, s := range rep.Killed {
			fmt.Fprintf(stdout, "  reaped %s\n", s)
		}
	}
	fmt.Fprintf(stdout, "skipped: live=%d foreign=%d no-pid=%d; errors=%d\n",
		rep.SkippedLive, rep.SkippedForeign, rep.SkippedUnparseable, len(rep.Errors))
	for _, e := range rep.Errors {
		fmt.Fprintf(stderr, "evolve gc: error: %s\n", e)
	}

	// Also sweep whole per-run tmux sockets a crashed loop left behind.
	var srep swarm.OrphanSocketReport
	if *dryRun {
		noopKill := func(_ context.Context, _ string) error { return nil }
		srep = swarm.ReapOrphanSockets(ctx, swarm.ExecListBridgeSockets, swarm.ExecPidAlive, noopKill)
		fmt.Fprintf(stdout, "evolve gc --dry-run: %d dead per-run socket(s) would be reaped\n", len(srep.Killed))
	} else {
		srep = swarm.ExecReapOrphanSockets(ctx)
		fmt.Fprintf(stdout, "evolve gc: reaped %d dead per-run socket(s)\n", len(srep.Killed))
	}
	for _, s := range srep.Killed {
		fmt.Fprintf(stdout, "  socket %s\n", s)
	}
	for _, e := range srep.Errors {
		fmt.Fprintf(stderr, "evolve gc: socket error: %s\n", e)
	}

	wrc := gcWorkspaceSweep(*projectRoot, *dryRun, stdout, stderr)

	if len(rep.Errors) > 0 || len(srep.Errors) > 0 || wrc != 0 {
		return 1
	}
	return 0
}

// gcWorkspaceSweep runs the worktree+branch backlog sweep for an operator.
//
// Safety is inherited from the planner, not re-implemented here: PlanWorktrees
// only plans deletes for merged, clean, dead worktrees/branches and flags the
// rest; this command prints flags but never upgrades one to a deletion.
// Returns non-zero only when the plan itself failed.
func gcWorkspaceSweep(projectRoot string, dryRun bool, stdout, stderr io.Writer) int {
	if projectRoot == "" {
		if !dryRun {
			fmt.Fprintf(stderr, "evolve gc: mutating run refused: --project-root must be explicitly set\n")
			return 1
		}
		// For --dry-run only, cwd is allowed: unlike runWorktreeGC, a mutating
		// sweep requires explicit aim, not whatever repo we happen to be standing in.
		cwd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(stderr, "evolve gc: workspace sweep skipped: no --project-root and cwd is unreadable: %v\n", err)
			return 1
		}
		projectRoot = cwd
	}
	evolveDir := filepath.Join(projectRoot, ".evolve")
	pol, err := policy.Load(filepath.Join(evolveDir, "policy.json"))
	if err != nil {
		fmt.Fprintf(stderr, "evolve gc: WARN: policy load failed: %v; using zero-value gc policy\n", err)
	}
	var wpol gc.WorktreesPolicy
	if pol.GC != nil {
		wpol = pol.GC.Worktrees
	}
	opts := worktreeGCOptions(projectRoot, evolveDir, wpol)
	manifest, err := gc.PlanWorktrees(opts)
	if err != nil {
		fmt.Fprintf(stderr, "evolve gc: workspace sweep plan failed: %v\n", err)
		return 1
	}

	planned, flagged := gcWorktreeCounts(manifest)
	if dryRun {
		fmt.Fprintf(stdout, "evolve gc --dry-run: %d workspace item(s) would be reaped (%d flagged for manual review)\n", planned, flagged)
	} else {
		fmt.Fprintf(stdout, "evolve gc: applying workspace sweep — %d item(s) (%d flagged for manual review)\n", planned, flagged)
	}
	for _, it := range manifest.Items {
		switch it.Action {
		case gc.WorktreeActionFlagDirty, gc.WorktreeActionFlagUnmerged:
			// Never prefixed WOULD-: a flag is not a planned mutation in
			// either mode — it is work this sweep is refusing to touch.
			fmt.Fprintf(stdout, "  %s %s (%s)\n", strings.ToUpper(string(it.Action)), gcWorktreeItemLabel(it), it.Reason)
		default:
			fmt.Fprintf(stdout, "  WOULD-%s %s (%s)\n", strings.ToUpper(string(it.Action)), gcWorktreeItemLabel(it), it.Reason)
		}
	}
	if dryRun {
		return 0
	}
	if err := gc.ApplyWorktrees(opts, manifest); err != nil {
		// Partial application is NORMAL: ApplyWorktrees joins per-item
		// refusals (a branch that became unmerged, a worktree that went dirty
		// since planning). Report and continue — refusing to reap is the
		// safe direction, so it is not a command failure.
		fmt.Fprintf(stderr, "evolve gc: workspace sweep partial: %v\n", err)
	}
	fmt.Fprintf(stdout, "evolve gc: workspace sweep applied\n")
	return 0
}

// gcWorktreeItemLabel renders an item's identity: branch-only backlog entries
// have no worktree dir, so naming the path unconditionally would print an
// empty field for exactly the entries the branch sweep is about.
func gcWorktreeItemLabel(it gc.WorktreeItem) string {
	if it.Path == "" {
		return "branch=" + it.Branch
	}
	if it.Branch == "" {
		return "path=" + it.Path
	}
	return "branch=" + it.Branch + " path=" + it.Path
}

// gcWorktreeCounts splits a manifest into mutating (planned) and flag-only
// (never touched) items.
func gcWorktreeCounts(m gc.WorktreeManifest) (planned, flagged int) {
	for _, it := range m.Items {
		switch it.Action {
		case gc.WorktreeActionFlagDirty, gc.WorktreeActionFlagUnmerged:
			flagged++
		default:
			planned++
		}
	}
	return planned, flagged
}
