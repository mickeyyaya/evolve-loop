package swarm

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

// WorkerProvisioner creates and removes the per-worker git worktrees + the
// shared integration branch a WRITER swarm needs. It is defined HERE (in the
// swarm package, consumed here) rather than widening core.WorktreeProvisioner —
// accept-interface-where-used keeps the swarm self-contained with zero blast
// radius on the core per-cycle provisioner. Readers need no provisioning, so the
// dispatcher only calls this for writer swarms.
type WorkerProvisioner interface {
	// CreateIntegration provisions the shared integration branch+worktree
	// (cycle-<N>-integration off HEAD). Idempotent. Returns its path.
	CreateIntegration(ctx context.Context, projectRoot string, cycle int) (string, error)
	// CreateWorker provisions a worker worktree (cycle-<N>-<workerID>) branched
	// off the integration branch, so workers start from the agreed base.
	// Idempotent. Returns its path.
	CreateWorker(ctx context.Context, projectRoot string, cycle int, workerID, integrationBranch string) (string, error)
	// Cleanup removes a worktree (best-effort; missing is not an error).
	Cleanup(ctx context.Context, projectRoot, worktree string) error
}

// gitWorkerProvisioner is the production WorkerProvisioner. It mirrors the proven
// `git worktree add -B <branch> <wt> <base>` flow from core.gitWorktree (-B is
// idempotent + concurrency-safe) and uses NAMED branches (not --detach) so the
// merge-train and ship can resolve them via `git symbolic-ref --short HEAD`.
//
// Branch/worktree naming:
//   - integration: branch cycle-<N>-integration, worktree <base>/cycle-<N>-integration
//   - worker:      branch cycle-<N>-<workerID>,  worktree <base>/cycle-<N>-<workerID>
//
// base = EVOLVE_WORKTREE_BASE or <root>/.evolve/worktrees (same as core).
type gitWorkerProvisioner struct {
	// LinkGuardDeps is an optional hook to make a fresh worktree self-sufficient
	// for the trust-kernel hooks (symlink binary + .evolve state). Injected so
	// the swarm package does not import core; cmd wiring supplies core's
	// equivalent. Nil = skip (tests / non-hooked environments).
	LinkGuardDeps func(worktree, projectRoot string)

	// newGit builds the gitexec.Git for a given -C dir. Nil => production
	// gitexec.Default. Injected by tests to fake every git call. A factory (not a
	// single Git) because one provision op spans two dirs: the worktree's own dir
	// for the reuse rev-parse probe, the project root for worktree add/remove.
	newGit func(dir string) gitexec.Git
}

// git returns the gitexec.Git rooted at dir, defaulting to the production
// runner when newGit is unset.
func (g gitWorkerProvisioner) git(dir string) gitexec.Git {
	if g.newGit != nil {
		return g.newGit(dir)
	}
	return gitexec.Default(dir)
}

// gitFailReason renders a one-line failure reason from a gitexec.Capture result
// known to be a failure (err != nil || code != 0): the unrecoverable error if
// present, else the non-zero exit code. Shared by provision + mergetrain.
func gitFailReason(code int, err error) string {
	if err != nil {
		return err.Error()
	}
	return fmt.Sprintf("exit %d", code)
}

// NewGitWorkerProvisioner returns the production provisioner. linkGuardDeps may
// be nil (skipped) — supply core.LinkGuardDeps at the composition root.
func NewGitWorkerProvisioner(linkGuardDeps func(worktree, projectRoot string)) WorkerProvisioner {
	return gitWorkerProvisioner{LinkGuardDeps: linkGuardDeps}
}

func worktreeBase(projectRoot string) (string, error) {
	if b := os.Getenv("EVOLVE_WORKTREE_BASE"); b != "" {
		if !filepath.IsAbs(b) {
			return "", fmt.Errorf("worktree base must be absolute: %s", b)
		}
		return b, nil
	}
	if !filepath.IsAbs(projectRoot) {
		return "", fmt.Errorf("worktree base: project root must be absolute: %s", projectRoot)
	}
	return filepath.Join(projectRoot, ".evolve", "worktrees"), nil
}

func (g gitWorkerProvisioner) CreateIntegration(ctx context.Context, projectRoot string, cycle int) (string, error) {
	branch := fmt.Sprintf("cycle-%d-integration", cycle)
	return g.addWorktree(ctx, projectRoot, branch, "HEAD")
}

func (g gitWorkerProvisioner) CreateWorker(ctx context.Context, projectRoot string, cycle int, workerID, integrationBranch string) (string, error) {
	base := integrationBranch
	if base == "" {
		base = "HEAD"
	}
	branch := fmt.Sprintf("cycle-%d-%s", cycle, workerID)
	return g.addWorktree(ctx, projectRoot, branch, base)
}

// addWorktree runs the idempotent `git worktree add -B <branch> <wt> <base>`,
// reusing an existing valid worktree (and tearing down a stale stub first).
func (g gitWorkerProvisioner) addWorktree(ctx context.Context, projectRoot, branch, base string) (string, error) {
	root, err := worktreeBase(projectRoot)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", fmt.Errorf("worktree base: %w", err)
	}
	wt := filepath.Join(root, branch)

	// Reuse an existing VALID worktree; tear down a stale stub git rejects.
	// Validity needs BOTH probes: a `.git` entry at the worktree root (a plain
	// stub dir inside the parent repo passes rev-parse by walking up to the
	// parent's .git, silently "reusing" a non-worktree — cycle-283 finding) and
	// a rev-parse to reject a corrupt/orphaned .git entry.
	if fi, err := os.Stat(wt); err == nil && fi.IsDir() {
		_, gitEntryErr := os.Stat(filepath.Join(wt, ".git"))
		if gitEntryErr == nil && g.git(wt).Run(ctx, "rev-parse", "--git-dir") == nil {
			g.link(wt, projectRoot)
			return wt, nil
		}
		_ = g.git(projectRoot).Run(ctx, "worktree", "remove", "--force", wt)
		_ = os.RemoveAll(wt)
	}

	if _, stderr, code, err := g.git(projectRoot).Capture(ctx, "worktree", "add", "-B", branch, wt, base); err != nil || code != 0 {
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
		// Best-effort but surfaced: a failed remove leaves an orphan worktree.
		fmt.Fprintf(os.Stderr, "[swarm] WARN worktree remove %s failed: %s: %s\n", worktree, gitFailReason(code, err), strings.TrimSpace(stderr))
	}
	_ = os.RemoveAll(worktree)
	return nil
}
