package swarm

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
}

// NewGitWorkerProvisioner returns the production provisioner. linkGuardDeps may
// be nil (skipped) — supply core.LinkGuardDeps at the composition root.
func NewGitWorkerProvisioner(linkGuardDeps func(worktree, projectRoot string)) WorkerProvisioner {
	return gitWorkerProvisioner{LinkGuardDeps: linkGuardDeps}
}

func worktreeBase(projectRoot string) string {
	if b := os.Getenv("EVOLVE_WORKTREE_BASE"); b != "" {
		return b
	}
	return filepath.Join(projectRoot, ".evolve", "worktrees")
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
	root := worktreeBase(projectRoot)
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
		if gitEntryErr == nil && exec.CommandContext(ctx, "git", "-C", wt, "rev-parse", "--git-dir").Run() == nil {
			g.link(wt, projectRoot)
			return wt, nil
		}
		_ = exec.CommandContext(ctx, "git", "-C", projectRoot, "worktree", "remove", "--force", wt).Run()
		_ = os.RemoveAll(wt)
	}

	cmd := exec.CommandContext(ctx, "git", "-C", projectRoot, "worktree", "add", "-B", branch, wt, base)
	var eb bytes.Buffer
	cmd.Stderr = &eb
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git worktree add -B %s %s %s: %v: %s", branch, wt, base, err, eb.String())
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
	var eb bytes.Buffer
	cmd := exec.CommandContext(ctx, "git", "-C", projectRoot, "worktree", "remove", "--force", worktree)
	cmd.Stderr = &eb
	if err := cmd.Run(); err != nil {
		// Best-effort but surfaced: a failed remove leaves an orphan worktree.
		fmt.Fprintf(os.Stderr, "[swarm] WARN worktree remove %s failed: %v: %s\n", worktree, err, eb.String())
	}
	_ = os.RemoveAll(worktree)
	return nil
}
