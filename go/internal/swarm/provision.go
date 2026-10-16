package swarm

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/runscope"
)

type WorkerProvisioner interface {
	CreateIntegration(ctx context.Context, projectRoot string, cycle int) (string, error)
	CreateWorker(ctx context.Context, projectRoot string, cycle int, workerID, integrationBranch string) (string, error)
	Cleanup(ctx context.Context, projectRoot, worktree string) error
}

type gitWorkerProvisioner struct {
	LinkGuardDeps func(worktree, projectRoot string)

	newGit func(dir string) gitexec.Git

	baseOverride string

	retry gitexec.WorktreeAddRetry
}

func (g gitWorkerProvisioner) git(dir string) gitexec.Git {
	if g.newGit != nil {
		return g.newGit(dir)
	}
	return gitexec.Default(dir)
}

func gitFailReason(code int, err error) string {
	if err != nil {
		return err.Error()
	}
	return fmt.Sprintf("exit %d", code)
}

func NewGitWorkerProvisioner(linkGuardDeps func(worktree, projectRoot string), baseOverride string) WorkerProvisioner {
	return gitWorkerProvisioner{
		LinkGuardDeps: linkGuardDeps,
		baseOverride:  baseOverride,
		retry: gitexec.WorktreeAddRetry{
			Retryable: gitexec.RetryableWorktreeAddFailure,
			OnRetry: func(attempt, attempts, code int, _ string) {
				fmt.Fprintf(os.Stderr, "[swarm] retry %d/%d: git worktree add after retryable rc=%d\n", attempt, attempts-1, code)
			},
		},
	}
}

func worktreeBase(baseOverride, projectRoot string) (string, error) {
	if baseOverride != "" {
		if !filepath.IsAbs(baseOverride) {
			return "", fmt.Errorf("worktree base must be absolute: %s", baseOverride)
		}
		return baseOverride, nil
	}
	if !filepath.IsAbs(projectRoot) {
		return "", fmt.Errorf("worktree base: project root must be absolute: %s", projectRoot)
	}
	return filepath.Join(projectRoot, ".evolve", "worktrees"), nil
}

func (g gitWorkerProvisioner) CreateIntegration(ctx context.Context, projectRoot string, cycle int) (string, error) {
	branch := runscope.New(runscope.LaneFromRoot(projectRoot), "", cycle).IntegrationBranch()
	return g.addWorktree(ctx, projectRoot, branch, "HEAD")
}

func (g gitWorkerProvisioner) CreateWorker(ctx context.Context, projectRoot string, cycle int, workerID, integrationBranch string) (string, error) {
	base := integrationBranch
	if base == "" {
		base = "HEAD"
	}
	branch := runscope.New(runscope.LaneFromRoot(projectRoot), "", cycle).WorkerBranch(workerID)
	return g.addWorktree(ctx, projectRoot, branch, base)
}

func (g gitWorkerProvisioner) addWorktree(ctx context.Context, projectRoot, branch, base string) (string, error) {
	root, err := worktreeBase(g.baseOverride, projectRoot)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", fmt.Errorf("worktree base: %w", err)
	}
	wt := filepath.Join(root, branch)

	if fi, err := os.Stat(wt); err == nil && fi.IsDir() {
		_, gitEntryErr := os.Stat(filepath.Join(wt, ".git"))
		if gitEntryErr == nil && g.git(wt).Run(ctx, "rev-parse", "--git-dir") == nil {
			g.link(wt, projectRoot)
			return wt, nil
		}
		_ = g.git(projectRoot).Run(ctx, "worktree", "remove", "--force", wt)
		_ = os.RemoveAll(wt)
	}

	if _, stderr, code, err := g.git(projectRoot).AddWorktreeWithRetry(ctx, g.retry, "-B", branch, wt, base); err != nil || code != 0 {
		return "", fmt.Errorf("git worktree add -B %s %s %s: %s: %s", branch, wt, base, gitFailReason(code, err), strings.TrimSpace(stderr))
	}
	g.link(wt, projectRoot)
	return wt, nil
}

func (g gitWorkerProvisioner) link(worktree, projectRoot string) {
	if g.LinkGuardDeps != nil {
		g.LinkGuardDeps(worktree, projectRoot)
	}
}

func (g gitWorkerProvisioner) Cleanup(ctx context.Context, projectRoot, worktree string) error {
	if worktree == "" {
		return nil
	}
	if _, stderr, code, err := g.git(projectRoot).Capture(ctx, "worktree", "remove", "--force", worktree); err != nil || code != 0 {
		fmt.Fprintf(os.Stderr, "[swarm] WARN worktree remove %s failed: %s: %s\n", worktree, gitFailReason(code, err), strings.TrimSpace(stderr))
	}
	_ = os.RemoveAll(worktree)
	g.deleteBranch(ctx, projectRoot, worktree)
	return nil
}

func (g gitWorkerProvisioner) deleteBranch(ctx context.Context, projectRoot, worktree string) {
	branch := filepath.Base(worktree)
	if !strings.HasPrefix(branch, "cycle-") {
		return
	}
	if _, stderr, code, err := g.git(projectRoot).Capture(ctx, "branch", "-d", branch); err != nil || code != 0 {
		fmt.Fprintf(os.Stderr, "[swarm] WARN branch -d %s failed (likely unmerged — left in place): %s: %s\n", branch, gitFailReason(code, err), strings.TrimSpace(stderr))
	}
}
