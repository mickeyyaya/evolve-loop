package ship

import (
	"context"
	"io"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/shipmanifest"
)

func earlyFastForwardApplies(opts *Options) bool {
	return opts.Class == ClassCycle && !opts.DryRun && !opts.PushOnly && opts.envBool(ipcenv.FleetKey) && !opts.deliverEnforced()
}

func checkEarlyFastForward(ctx context.Context, opts *Options) error {
	if !earlyFastForwardApplies(opts) {
		return nil
	}
	s, commit, ok := earlyLane(ctx, opts)
	if !ok || !s.auditedOnLaneBase() || s.planCatchUp().advances() || !s.carriesWork() {
		return nil
	}
	return s.checkFastForward(commit)
}

func earlyLane(ctx context.Context, opts *Options) (*worktreeShip, string, bool) {
	worktree, fromWorktree, err := landingTree(opts)
	if err != nil || !fromWorktree {
		return nil, "", false
	}
	branch, err := currentBranch(ctx, opts)
	if err != nil || branch == "" {
		return nil, "", false
	}
	s := newWorktreeShip(ctx, opts, &RunResult{}, branch, worktree)
	if s.resolveCycleBranch() != nil {
		return nil, "", false
	}
	commit, err := s.landingCommit("")
	return s, commit, err == nil
}

func (s *worktreeShip) auditedOnLaneBase() bool {
	runID, err := runIDForShip(s.opts)
	if err != nil {
		return false
	}
	entry, err := latestRunAudit(s.opts, runID)
	return err == nil && entry.AuditedBase() == s.opts.WorktreeBaseSHA
}

func (s *worktreeShip) carriesWork() bool {
	read := func(args ...string) (string, error) {
		return captureGitOutputAtDir(s.ctx, s.opts, s.worktree, args...)
	}
	sel, err := shipmanifest.Select(read, s.worktree, s.opts.WorkspacePath)
	if err != nil {
		return false
	}
	if len(sel.Paths) > 0 {
		return true
	}
	diff := []string{"-C", s.worktree, "diff", "--quiet", "HEAD", "--", "."}
	for _, p := range binaryChurnPaths(s.opts) {
		diff = append(diff, ":(exclude)"+p)
	}
	exit, err := s.opts.run(s.ctx, "git", diff, io.Discard, io.Discard)
	if err != nil {
		return false
	}
	if exit != 0 {
		return true
	}
	ahead, err := captureGitOutputAtDir(s.ctx, s.opts, s.worktree, "rev-list", "--count", s.branch+"..HEAD")
	return err == nil && strings.TrimSpace(ahead) != "0"
}
