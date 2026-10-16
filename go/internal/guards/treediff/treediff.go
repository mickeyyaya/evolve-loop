// Package treediff is the post-phase guard that flags main-tree writes that escaped the worktree.
// See docs/architecture/packages/internal-guards-treediff.md.
package treediff

import (
	"context"
	"fmt"
)

type GitDirtyFn func(ctx context.Context, repoRoot string) ([]string, error)

type Guard struct {
	gitDirty GitDirtyFn
}

func New(gitDirty GitDirtyFn) *Guard {
	return &Guard{gitDirty: gitDirty}
}

func (g *Guard) Snapshot(ctx context.Context, repoRoot string) ([]string, error) {
	if g == nil || g.gitDirty == nil {
		return nil, nil
	}
	return g.gitDirty(ctx, repoRoot)
}

type CheckResult struct {
	Leaked         []string
	SnapshotMissed bool
}

func (r CheckResult) OK() bool { return len(r.Leaked) == 0 && !r.SnapshotMissed }

func (r CheckResult) Error(phase, worktree string) error {
	if r.OK() {
		return nil
	}
	if r.SnapshotMissed {
		return fmt.Errorf("tree-diff guard: pre-phase snapshot failed for %s — leak detection skipped (continuing; trust kernel falls back to Claude-only PreToolUse)", phase)
	}
	return fmt.Errorf("tree-diff guard: phase %q wrote to the main tree outside its worktree %q — leaked paths: %v",
		phase, worktree, r.Leaked)
}

func (g *Guard) Check(ctx context.Context, repoRoot string, before []string) CheckResult {
	if g == nil || g.gitDirty == nil {
		return CheckResult{}
	}
	after, err := g.gitDirty(ctx, repoRoot)
	if err != nil {
		return CheckResult{SnapshotMissed: true}
	}
	beforeSet := make(map[string]struct{}, len(before))
	for _, p := range before {
		beforeSet[p] = struct{}{}
	}
	var leaked []string
	for _, p := range after {
		if _, was := beforeSet[p]; was {
			continue
		}
		leaked = append(leaked, p)
	}
	return CheckResult{Leaked: leaked}
}
