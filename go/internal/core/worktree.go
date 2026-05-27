package core

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

// worktree.go — per-cycle git worktree provisioning for the Go orchestrator.
//
// Source-writing phases (tdd, build) run in an isolated per-cycle worktree so a
// failed or buggy cycle never mutates the live working tree. The bash
// run-cycle.sh provisioned these; the v11 Go port dropped it, which left
// cs.ActiveWorktree empty and the role-gate's only source-write allowance
// (phase==build && ActiveWorktree!="") permanently unsatisfiable — i.e. no
// phase could write code. This restores provisioning behind an injected seam
// so RunCycle stays unit-testable without real git. See ADR-0027.

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

// gitWorktree is the production provisioner: `git worktree add --detach
// <base>/cycle-N HEAD`, base = EVOLVE_WORKTREE_BASE or <root>/.evolve/worktrees.
// Mirrors `evolve worktree create|cleanup` (cmd_worktree.go).
type gitWorktree struct{}

func (gitWorktree) base(projectRoot string) string {
	if b := os.Getenv("EVOLVE_WORKTREE_BASE"); b != "" {
		return b
	}
	return filepath.Join(projectRoot, ".evolve", "worktrees")
}

func (g gitWorktree) Create(projectRoot string, cycle int) (string, error) {
	base := g.base(projectRoot)
	if err := os.MkdirAll(base, 0o755); err != nil {
		return "", fmt.Errorf("worktree base: %w", err)
	}
	wt := filepath.Join(base, "cycle-"+strconv.Itoa(cycle))
	// Idempotent reuse across resume/retry — but only if it is still a VALID
	// git worktree. A pruned ref can leave a stale directory os.Stat sees but
	// git rejects; reusing it would make every commit inside fail. Verify, and
	// tear down a stale stub before recreating.
	if fi, err := os.Stat(wt); err == nil && fi.IsDir() {
		if exec.Command("git", "-C", wt, "rev-parse", "--git-dir").Run() == nil {
			return wt, nil
		}
		_ = exec.Command("git", "-C", projectRoot, "worktree", "remove", "--force", wt).Run()
		_ = os.RemoveAll(wt)
	}
	// Named branch (NOT --detach): worktree-aware ship resolves the cycle branch
	// via `git symbolic-ref --short HEAD` and ff-merges it to main — a detached
	// HEAD yields an empty branch and ship fails. -B creates or resets cycle-<N>
	// to HEAD, tolerating a leftover branch from a prior attempt at this cycle.
	branch := "cycle-" + strconv.Itoa(cycle)
	cmd := exec.Command("git", "-C", projectRoot, "worktree", "add", "-B", branch, wt, "HEAD")
	var eb bytes.Buffer
	cmd.Stderr = &eb
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git worktree add -B %s %s: %v: %s", branch, wt, err, eb.String())
	}
	return wt, nil
}

func (gitWorktree) Cleanup(projectRoot, worktree string) error {
	if worktree == "" {
		return nil
	}
	var eb bytes.Buffer
	cmd := exec.Command("git", "-C", projectRoot, "worktree", "remove", "--force", worktree)
	cmd.Stderr = &eb
	if err := cmd.Run(); err != nil {
		// Best-effort, but surface it: a failed remove leaves an orphaned
		// worktree that would accumulate silently.
		fmt.Fprintf(os.Stderr, "[worktree] WARN remove %s failed: %v: %s\n", worktree, err, eb.String())
	}
	_ = os.RemoveAll(worktree) // clear any leftover stub git left behind
	return nil
}

// WorktreePhase reports whether a phase writes source into the cycle worktree
// (and therefore needs cwd=worktree + a role-gate write allowance there). Only
// tdd (RED *_test.go) and build (production code) write source; every other
// phase writes only its artifact into the absolute workspace path. Exported so
// the role-gate (guards) and the orchestrator share one definition.
func WorktreePhase(p Phase) bool {
	return p == PhaseTDD || p == PhaseBuild
}
