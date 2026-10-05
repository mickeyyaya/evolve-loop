package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

type staleLane struct {
	leaf, path, branch string
	cycle              int
	keep               string
}

type laneBindings struct {
	byBranch, byLeaf map[string]string
}

func runWorktreeCleanupStale(projectRoot, base string, apply bool, stdout, stderr io.Writer) int {
	ctx := context.Background()
	lanes, err := planStaleLanes(ctx, projectRoot, base, time.Now())
	if err != nil {
		fmt.Fprintf(stderr, "evolve worktree cleanup --stale: %v\n", err)
		return 1
	}
	if len(lanes) == 0 {
		fmt.Fprintf(stdout, "no cycle worktrees under %s\n", base)
		return 0
	}
	failed := false
	for _, l := range lanes {
		switch {
		case l.keep != "":
			fmt.Fprintf(stdout, "%s kept: %s path=%s\n", l.leaf, l.keep, l.path)
		case !apply:
			fmt.Fprintf(stdout, "%s would-remove path=%s branch=%s: sealed, unbound, no live lease, clean\n", l.leaf, l.path, l.branch)
		default:
			failed = removeStaleLane(ctx, projectRoot, l, stdout, stderr) || failed
		}
	}
	if failed {
		return 1
	}
	return 0
}

func planStaleLanes(ctx context.Context, projectRoot, base string, now time.Time) ([]staleLane, error) {
	porcelain, err := gitexec.Default(projectRoot).Output(ctx, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("list worktrees: %w", err)
	}
	bindings, err := readLaneBindings(projectRoot)
	if err != nil {
		return nil, err
	}
	resolvedBase := resolvedPath(base)
	var lanes []staleLane
	for _, wt := range parseWorktreeList(porcelain) {
		leaf := filepath.Base(wt.path)
		n, ok := gc.LeafCycleNumber(leaf)
		if !ok || !strings.HasPrefix(leaf, "cycle-") || resolvedPath(filepath.Dir(wt.path)) != resolvedBase {
			continue
		}
		l := staleLane{leaf: leaf, path: filepath.Join(base, leaf), branch: wt.branch, cycle: n}
		if l.keep, err = staleKeepReason(ctx, projectRoot, l, bindings, now); err != nil {
			return nil, err
		}
		lanes = append(lanes, l)
	}
	return lanes, nil
}

func staleKeepReason(ctx context.Context, projectRoot string, l staleLane, b laneBindings, now time.Time) (string, error) {
	if !dossier.ClosedOut(projectRoot, l.cycle) {
		return fmt.Sprintf("cycle %d not sealed", l.cycle), nil
	}
	if scope, ok := b.bound(l); ok {
		return "named by continuation binding " + scope, nil
	}
	if lease, ok := liveCycleLease(projectRoot, l.cycle, now); ok {
		return fmt.Sprintf("fresh run lease (run %s, pid %d)", lease.RunID, lease.OwnerPID), nil
	}
	if _, err := os.Stat(l.path); err != nil {
		return fmt.Sprintf("worktree directory unreadable (%v); `evolve worktree cleanup` prunes a missing one", err), nil
	}
	dirty, err := gitexec.Default(l.path).DirtyPaths(ctx)
	if err != nil {
		return "", fmt.Errorf("%s: %w", l.leaf, err)
	}
	if len(dirty) > 0 {
		return fmt.Sprintf("dirty worktree (%d uncommitted path(s))", len(dirty)), nil
	}
	return "", nil
}

func removeStaleLane(ctx context.Context, projectRoot string, l staleLane, stdout, stderr io.Writer) bool {
	if lease, ok := liveCycleLease(projectRoot, l.cycle, time.Now()); ok {
		fmt.Fprintf(stdout, "%s kept: run lease became fresh before removal (run %s) path=%s\n", l.leaf, lease.RunID, l.path)
		return false
	}
	g := gitexec.Default(projectRoot)
	if _, gitErr, code, err := g.Capture(ctx, "worktree", "remove", l.path); err != nil || code != 0 {
		fmt.Fprintf(stderr, "evolve worktree cleanup --stale: %s: git worktree remove rc=%d err=%v: %s\n", l.leaf, code, err, strings.TrimSpace(gitErr))
		return true
	}
	branch := "none"
	if l.branch != "" {
		branch = l.branch + " deleted"
		if _, gitErr, code, err := g.Capture(ctx, "branch", "-d", l.branch); err != nil || code != 0 {
			branch = fmt.Sprintf("%s left in place, not merged into HEAD (%s)", l.branch, strings.TrimSpace(gitErr))
		}
	}
	fmt.Fprintf(stdout, "%s removed path=%s branch=%s\n", l.leaf, l.path, branch)
	return false
}

func readLaneBindings(projectRoot string) (laneBindings, error) {
	entries, err := continuation.ListRegistryEntries(projectRoot)
	if err != nil {
		return laneBindings{}, err
	}
	scopes := make([]string, 0, len(entries))
	for scope := range entries {
		scopes = append(scopes, scope)
	}
	sort.Strings(scopes)
	b := laneBindings{byBranch: map[string]string{}, byLeaf: map[string]string{}}
	for _, scope := range scopes {
		c := entries[scope]
		if c.Branch != "" {
			b.byBranch[c.Branch] = scope
		}
		if c.Worktree != "" {
			b.byLeaf[filepath.Base(c.Worktree)] = scope
		}
	}
	return b, nil
}

func (b laneBindings) bound(l staleLane) (string, bool) {
	if scope, ok := b.byBranch[l.branch]; ok && l.branch != "" {
		return scope, true
	}
	scope, ok := b.byLeaf[l.leaf]
	return scope, ok
}

func liveCycleLease(projectRoot string, cycle int, now time.Time) (runlease.Lease, bool) {
	return runlease.LiveOwner(filepath.Join(projectRoot, ".evolve", "runs", fmt.Sprintf("cycle-%d", cycle)), now)
}

func liveLaneRef(projectRoot string, now time.Time) func(ref string) bool {
	return func(ref string) bool {
		n, ok := gc.LeafCycleNumber(ref)
		if !ok {
			return false
		}
		_, live := liveCycleLease(projectRoot, n, now)
		return live
	}
}

type worktreeListEntry struct{ path, branch string }

func parseWorktreeList(porcelain string) []worktreeListEntry {
	var out []worktreeListEntry
	for _, line := range strings.Split(porcelain, "\n") {
		if p, ok := strings.CutPrefix(line, "worktree "); ok {
			out = append(out, worktreeListEntry{path: strings.TrimSpace(p)})
			continue
		}
		if b, ok := strings.CutPrefix(line, "branch refs/heads/"); ok && len(out) > 0 {
			out[len(out)-1].branch = strings.TrimSpace(b)
		}
	}
	return out
}

func resolvedPath(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return filepath.Clean(p)
}
