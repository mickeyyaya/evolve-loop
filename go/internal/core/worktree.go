package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/runscope"
)

// WorktreeProvisioner creates and removes the per-cycle worktree. Injected via
// WithWorktreeProvisioner; the default is gitWorktree (real `git worktree`).
type WorktreeProvisioner interface {
	// Create provisions (or reuses) the cycle's worktree and returns its
	// absolute path. Idempotent: an existing worktree for the cycle is reused.
	Create(projectRoot string, cycle int) (string, error)
	// Cleanup removes the worktree. Best-effort; a missing worktree is not an
	// error. Empty worktree path is a no-op.
	Cleanup(projectRoot, worktree string) error
}

type gitWorktree struct {
	baseOverride string
}

func (g gitWorktree) base(projectRoot string) string {
	if g.baseOverride != "" {
		return g.baseOverride
	}
	return filepath.Join(projectRoot, ".evolve", "worktrees")
}

func (g gitWorktree) Create(projectRoot string, cycle int) (string, error) {
	base := g.base(projectRoot)
	if !filepath.IsAbs(base) {
		return "", fmt.Errorf("worktree base must be absolute: %s", base)
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		return "", fmt.Errorf("worktree base: %w", err)
	}
	rs := runscope.New(runscope.LaneFromRoot(projectRoot), "", cycle)
	wt := rs.WorktreeDir(base)
	if fi, err := os.Stat(wt); err == nil && fi.IsDir() {
		valid := gitexec.Git{Dir: wt, Exec: gitRunner}.Run(context.Background(), "rev-parse", "--git-dir") == nil
		if valid {
			if _, cerr := ensureCleanWorktree(context.Background(), wt, projectRoot, cycle); cerr != nil {
				return "", fmt.Errorf("worktree reuse (cycle %d): %w", cycle, cerr)
			}
			linkGuardDeps(wt, projectRoot, cycle)
			return wt, nil
		}
		_ = gitexec.Git{Dir: projectRoot, Exec: gitRunner}.Run(context.Background(), "worktree", "remove", "--force", wt)
		_ = os.RemoveAll(wt)
	}
	branch := rs.CycleBranch()
	startRef, err := laneStartRef(context.Background(), projectRoot)
	if err != nil {
		return "", fmt.Errorf("worktree base ref (cycle %d): %w", cycle, err)
	}
	_, stderr, code, err := gitexec.Git{Dir: projectRoot, Exec: gitRunner}.AddWorktreeWithRetry(
		context.Background(), worktreeAddRetry(branch), "-B", branch, wt, startRef)
	if err != nil || code != 0 {
		return "", fmt.Errorf("git worktree add -B %s %s %s: rc=%d err=%v: %s", branch, wt, startRef, code, err, stderr)
	}
	linkGuardDeps(wt, projectRoot, cycle)
	return wt, nil
}

// worktreeAddRetrySleep is the inter-attempt backoff clock — a seam so the
// package tests count sleeps instead of paying them.
var worktreeAddRetrySleep = time.Sleep

func worktreeAddRetry(branch string) gitexec.WorktreeAddRetry {
	return gitexec.WorktreeAddRetry{
		Sleep:     func(d time.Duration) { worktreeAddRetrySleep(d) },
		Retryable: gitexec.RetryableWorktreeAddFailure,
		OnRetry: func(attempt, attempts, code int, _ string) {
			fmt.Fprintf(os.Stderr, "[worktree] retry %d/%d: git worktree add -B %s after retryable rc=%d\n",
				attempt, attempts-1, branch, code)
		},
	}
}

func laneStartRef(ctx context.Context, projectRoot string) (string, error) {
	git := gitexec.Git{Dir: projectRoot, Exec: gitRunner}
	url, _, code, err := git.Capture(ctx, "config", "--get", "remote.origin.url")
	if err != nil || code != 0 || strings.TrimSpace(url) == "" {
		return "HEAD", nil // remoteless repo — documented fallback
	}
	branch := originDefaultBranch(ctx, git)
	if _, stderr, code, err := git.FetchOriginBranch(ctx, laneBaseFetchRetry(branch), branch); err != nil || code != 0 {
		return "", fmt.Errorf("git fetch origin %s: rc=%d err=%v: %s", branch, code, err, strings.TrimSpace(stderr))
	}
	return integrationHead(ctx, git, "origin/"+branch)
}

func laneBaseFetchRetry(branch string) gitexec.FetchRetry {
	return gitexec.FetchRetry{
		Sleep: func(d time.Duration) { worktreeAddRetrySleep(d) },
		OnRetry: func(attempt, attempts, code int, _ string) {
			fmt.Fprintf(os.Stderr, "[worktree] retry %d/%d: git fetch origin %s after ref-lock contention rc=%d\n",
				attempt, attempts-1, branch, code)
		},
	}
}

func integrationHead(ctx context.Context, git gitexec.Git, remote string) (string, error) {
	branch, _, bcode, berr := git.Capture(ctx, "symbolic-ref", "--quiet", "--short", "HEAD")
	if berr != nil || bcode != 0 || "origin/"+strings.TrimSpace(branch) != remote {
		return remote, nil // detached / feature checkout — the remote tip is the base
	}
	rel, err := git.RelationToRemote(ctx, remote)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN lane base: %v — basing on %s\n", err, remote)
		return remote, nil
	}
	switch rel.Kind {
	case gitexec.RelationAhead:
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN lane base: %s\n", rel)
		return rel.Local, nil
	case gitexec.RelationDiverged:
		return "", fmt.Errorf("lane base: %s", rel)
	default: // current or behind — the boundary fast-forwards a behind main
		return remote, nil
	}
}

// originDefaultBranch falls back to "main" when refs/remotes/origin/HEAD is
// absent; a wrong guess fails loudly on the unknown ref at worktree add rather
// than silently basing on the stale local tip.
func originDefaultBranch(ctx context.Context, git gitexec.Git) string {
	out, _, code, err := git.Capture(ctx, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
	if err != nil || code != 0 {
		return "main"
	}
	if b := strings.TrimPrefix(strings.TrimSpace(out), "origin/"); b != "" {
		return b
	}
	return "main"
}

// CreateFrom provisions the cycle worktree seeded from startRef instead of
// HEAD; an existing directory at this cycle number is always a stale
// collision and is recreated, never reused.
func (g gitWorktree) CreateFrom(projectRoot string, cycle int, startRef string) (string, error) {
	base := g.base(projectRoot)
	if !filepath.IsAbs(base) {
		return "", fmt.Errorf("worktree base must be absolute: %s", base)
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		return "", fmt.Errorf("worktree base: %w", err)
	}
	rs := runscope.New(runscope.LaneFromRoot(projectRoot), "", cycle)
	wt := rs.WorktreeDir(base)
	if _, err := os.Stat(wt); err == nil {
		_ = gitexec.Git{Dir: projectRoot, Exec: gitRunner}.Run(context.Background(), "worktree", "remove", "--force", wt)
		_ = os.RemoveAll(wt)
	}
	branch := rs.CycleBranch()
	_, stderr, code, err := gitexec.Git{Dir: projectRoot, Exec: gitRunner}.AddWorktreeWithRetry(
		context.Background(), worktreeAddRetry(branch), "-B", branch, wt, startRef)
	if err != nil || code != 0 {
		return "", fmt.Errorf("git worktree add -B %s %s %s: rc=%d err=%v: %s", branch, wt, startRef, code, err, stderr)
	}
	linkGuardDeps(wt, projectRoot, cycle)
	return wt, nil
}

func linkGuardDeps(worktree, projectRoot string, cycle int) {
	if self, err := os.Executable(); err == nil {
		if err := os.MkdirAll(filepath.Join(worktree, "go", "bin"), 0o755); err == nil {
			symlinkForce(self, filepath.Join(worktree, "go", "bin", "evolve"))
		}
	}
	if err := os.MkdirAll(filepath.Join(worktree, ".evolve"), 0o755); err == nil {
		symlinkForce(
			filepath.Join(RunWorkspacePath(projectRoot, cycle), RunStateFile),
			filepath.Join(worktree, ".evolve", "cycle-state.json"))
		for _, f := range []string{"state.json", "ledger.jsonl"} {
			symlinkForce(filepath.Join(projectRoot, ".evolve", f), filepath.Join(worktree, ".evolve", f))
		}
	}
}

func symlinkForce(src, dst string) {
	_ = os.Remove(dst)
	if err := os.Symlink(src, dst); err != nil {
		fmt.Fprintf(os.Stderr, "[worktree] WARN symlink %s → %s failed (guard hooks may not resolve): %v\n", dst, src, err)
	}
}

func (gitWorktree) Cleanup(projectRoot, worktree string) error {
	if worktree == "" {
		return nil
	}
	if inPlaceWorktree(worktree, projectRoot) {
		fmt.Fprintf(os.Stderr, "[worktree] WARN refusing to remove %s: it is the project root, not a provisioned worktree\n", worktree)
		return fmt.Errorf("worktree cleanup: %s is the project root — refusing to remove it", worktree)
	}
	_, stderr, code, err := gitexec.Git{Dir: projectRoot, Exec: gitRunner}.Capture(context.Background(), "worktree", "remove", "--force", worktree)
	if err != nil || code != 0 {
		fmt.Fprintf(os.Stderr, "[worktree] WARN remove %s failed (rc=%d): %v: %s\n", worktree, code, err, stderr)
	}
	_ = os.RemoveAll(worktree)
	deleteCycleBranch(projectRoot, worktree)
	return nil
}

func deleteCycleBranch(projectRoot, worktree string) {
	branch := filepath.Base(worktree)
	if !strings.HasPrefix(branch, "cycle-") {
		return
	}
	_, stderr, code, err := gitexec.Git{Dir: projectRoot, Exec: gitRunner}.Capture(context.Background(), "branch", "-d", branch)
	if err != nil || code != 0 {
		fmt.Fprintf(os.Stderr, "[worktree] WARN branch -d %s failed (rc=%d, likely unmerged — left in place): %v: %s\n", branch, code, err, stderr)
	}
}

// WorktreePhase reports whether a phase writes source into the cycle
// worktree: only tdd and build do; every other phase writes only its
// artifact into the absolute workspace path.
func WorktreePhase(p Phase) bool {
	return p == PhaseTDD || p == PhaseBuild
}

// LeakRecoverablePhase reports whether a phase runs with an active cycle
// worktree and is therefore eligible for leak recovery, a distinct axis from
// WorktreePhase's write permission.
func LeakRecoverablePhase(p Phase) bool {
	switch p {
	case PhaseTriage, PhaseAudit, PhaseScout, PhaseTDD, PhaseBuild, Phase("bug-reproduction"):
		return true
	default:
		return false
	}
}

func sameDirectory(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if fa, err := os.Stat(a); err == nil {
		if fb, err := os.Stat(b); err == nil && os.SameFile(fa, fb) {
			return true
		}
	}
	lexical := func(p string) string {
		if abs, err := filepath.Abs(p); err == nil {
			return filepath.Clean(abs)
		}
		return filepath.Clean(p)
	}
	return lexical(a) == lexical(b)
}

func inPlaceWorktree(worktree, projectRoot string) bool {
	return worktree != "" && projectRoot != "" && sameDirectory(worktree, projectRoot)
}
