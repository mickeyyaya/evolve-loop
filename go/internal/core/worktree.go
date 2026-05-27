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
			linkGuardDeps(wt, projectRoot)
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
	linkGuardDeps(wt, projectRoot)
	return wt, nil
}

// linkGuardDeps makes the worktree self-sufficient for the trust-kernel
// PreToolUse hooks. Those run `$CLAUDE_PROJECT_DIR/go/bin/evolve guard ...
// --evolve-dir $CLAUDE_PROJECT_DIR/.evolve`, and Claude Code pins
// CLAUDE_PROJECT_DIR to the cwd (the worktree) — it does NOT honor a pre-set
// value. The binary (gitignored go/bin/evolve) and the runtime .evolve state
// are absent in the fresh checkout, so the hooks fail and source phases stall.
// We symlink the LIVE dispatcher binary + the guard-read state files into the
// worktree so the hooks resolve to the real binary + current cycle-state.
// Best-effort: failures are non-fatal (the phase will surface a denial loudly).
func linkGuardDeps(worktree, projectRoot string) {
	// Binary: the running dispatcher's own executable carries the current guard
	// logic (incl. the worktree-phase role-gate allowance), avoiding a stale
	// in-tree go/bin/evolve.
	if self, err := os.Executable(); err == nil {
		if err := os.MkdirAll(filepath.Join(worktree, "go", "bin"), 0o755); err == nil {
			symlinkForce(self, filepath.Join(worktree, "go", "bin", "evolve"))
		}
	}
	// Guard-read state: point the worktree's .evolve files at the live main
	// copies so `--evolve-dir <worktree>/.evolve` reads real cycle-state. File-
	// level links (not a .evolve dir link) avoid any tree-walk recursion.
	if err := os.MkdirAll(filepath.Join(worktree, ".evolve"), 0o755); err == nil {
		for _, f := range []string{"cycle-state.json", "state.json", "ledger.jsonl"} {
			symlinkForce(filepath.Join(projectRoot, ".evolve", f), filepath.Join(worktree, ".evolve", f))
		}
	}
}

// symlinkForce replaces dst with a symlink to src (absolute), clearing any
// stale checkout file/link first. src may not yet exist (a dangling link that
// resolves once the target is written, e.g. cycle-state.json written just after
// provisioning) — that is fine.
func symlinkForce(src, dst string) {
	_ = os.Remove(dst)
	if err := os.Symlink(src, dst); err != nil {
		// Observable, not fatal: a missing binary link makes hooks fail loudly,
		// but a missing state link could let a guard read empty state and pass a
		// tool it should deny — so surface it rather than swallow silently.
		fmt.Fprintf(os.Stderr, "[worktree] WARN symlink %s → %s failed (guard hooks may not resolve): %v\n", dst, src, err)
	}
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
