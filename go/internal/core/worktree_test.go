package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorktreePhase(t *testing.T) {
	for _, p := range []Phase{PhaseTDD, PhaseBuild} {
		if !WorktreePhase(p) {
			t.Errorf("%s should be a worktree (source-writing) phase", p)
		}
	}
	for _, p := range []Phase{PhaseIntent, PhaseScout, PhaseTriage, PhaseBuildPlanner, PhaseAudit, PhaseShip, PhaseRetro, PhaseStart, PhaseEnd} {
		if WorktreePhase(p) {
			t.Errorf("%s should NOT be a worktree phase (writes only to workspace)", p)
		}
	}
}

// chdirTempNonGit chdirs the process into a fresh non-git temp dir for the
// duration of the test and restores the prior cwd on cleanup. The core
// package tests run sequentially (no t.Parallel), so the process-global cwd
// swap is safe. It serves two purposes for the relative-base guard tests:
//   - cwd is NOT inside a git repo, so an un-guarded (RED) build's
//     `git -C "." worktree add` fails cleanly instead of polluting the live
//     evolve-loop repo with a stray cycle-N branch/worktree.
//   - any relative base dir an un-guarded MkdirAll creates lands under this
//     temp dir, which t.TempDir() auto-removes — RED stays side-effect-free.
func chdirTempNonGit(t *testing.T) string {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	return dir
}

// TestGitWorktree_RelativeBaseRefused: a RELATIVE EVOLVE_WORKTREE_BASE must
// be refused with an "absolute" error BEFORE any MkdirAll/git runs, so a
// relative base can never silently create worktree dirs under the cwd.
// Mirrors the swarm/provision.go addWorktree guard added in cycle 294.
//
// RED today: gitWorktree.Create has no IsAbs guard — it MkdirAll's the
// relative base and then `git -C <root> worktree add` fails with a *git*
// message that does NOT mention "absolute", so the discriminating
// assertion fails.
func TestGitWorktree_RelativeBaseRefused(t *testing.T) {
	chdirTempNonGit(t)
	const relBase = "relative-base-probe" // relative → the bug class
	t.Setenv("EVOLVE_WORKTREE_BASE", relBase)

	g := gitWorktree{}
	wt, err := g.Create(".", 1)
	if err == nil {
		t.Fatalf("RED: relative EVOLVE_WORKTREE_BASE %q must be refused; got worktree %q, nil error", relBase, wt)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "absolute") {
		t.Errorf("RED: guard absent — error %q does not indicate the worktree base must be absolute", err.Error())
	}
	// No filesystem side effect: the guard must fire before MkdirAll, so the
	// relative base dir must not exist under the (temp) cwd.
	if _, statErr := os.Stat(relBase); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("RED: relative base dir %q was created (stat err=%v) — guard did not fire before MkdirAll", relBase, statErr)
	}
}

// TestGitWorktree_RelativeProjectRootRefused: with EVOLVE_WORKTREE_BASE
// unset and a relative projectRoot, base = "<root>/.evolve/worktrees" is
// itself relative and must also be refused. This is the live-default path
// (no env override) and the one that silently created dirs in the cwd.
//
// RED today: no guard → MkdirAll(".evolve/worktrees") then a git error
// lacking "absolute".
func TestGitWorktree_RelativeProjectRootRefused(t *testing.T) {
	chdirTempNonGit(t)
	t.Setenv("EVOLVE_WORKTREE_BASE", "") // empty → base() falls back to <root>/.evolve/worktrees

	g := gitWorktree{}
	wt, err := g.Create(".", 1)
	if err == nil {
		t.Fatalf("RED: relative projectRoot %q must yield a refused (non-absolute) base; got worktree %q, nil error", ".", wt)
	}
	if !strings.Contains(strings.ToLower(err.Error()), "absolute") {
		t.Errorf("RED: guard absent — error %q does not indicate the worktree base must be absolute", err.Error())
	}
	if _, statErr := os.Stat(filepath.Join(".evolve", "worktrees")); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("RED: .evolve/worktrees was created (stat err=%v) — guard did not fire before MkdirAll", statErr)
	}
}

// fakeWorktree records Create/Cleanup calls and returns a scripted path/err.
type fakeWorktree struct {
	createdCycles []int
	cleaned       []string
	path          string
	createErr     error
}

func (f *fakeWorktree) Create(_ string, cycle int) (string, error) {
	f.createdCycles = append(f.createdCycles, cycle)
	if f.createErr != nil {
		return "", f.createErr
	}
	return f.path, nil
}

func (f *fakeWorktree) Cleanup(_, worktree string) error {
	f.cleaned = append(f.cleaned, worktree)
	return nil
}

// TestOrchestrator_ProvisionsWorktree_PassesToSourcePhases proves the fix: the
// orchestrator provisions a worktree once per cycle, passes it as cwd to the
// source-writing phases (tdd, build) only, and cleans it up on exit.
func TestOrchestrator_ProvisionsWorktree_PassesToSourcePhases(t *testing.T) {
	st := &fakeStorage{state: State{LastCycleNumber: 9}} // cycle 10
	led := &fakeLedger{}
	runners := buildRunners(nil)
	wt := &fakeWorktree{path: "/tmp/wt/cycle-10"}
	o := NewOrchestrator(st, led, runners, WithWorktreeProvisioner(wt))

	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: "/tmp/p", GoalHash: "g"}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}

	if len(wt.createdCycles) != 1 || wt.createdCycles[0] != 10 {
		t.Fatalf("Create cycles = %v, want [10]", wt.createdCycles)
	}
	if len(wt.cleaned) != 1 || wt.cleaned[0] != "/tmp/wt/cycle-10" {
		t.Fatalf("Cleanup = %v, want [/tmp/wt/cycle-10]", wt.cleaned)
	}

	// Post-CB.1: EVERY phase runs with cwd=worktree — source writers so their
	// edits land where the role-gate permits, audit so its verification
	// commands inspect the builder's pending work (issue #9), and the
	// read-only spine so no phase subprocess has the live main tree as cwd.
	for _, p := range []Phase{PhaseTDD, PhaseBuild, PhaseAudit, PhaseScout, PhaseTriage} {
		fr := runners[p].(*fakeRunner)
		if len(fr.requests) == 0 {
			t.Fatalf("phase %s never ran", p)
		}
		if got := fr.requests[0].Worktree; got != "/tmp/wt/cycle-10" {
			t.Errorf("phase %s Worktree = %q, want /tmp/wt/cycle-10", p, got)
		}
	}
}

// TestOrchestrator_WorktreeProvisionFailure_BestEffort proves provisioning is
// best-effort: on Create failure the cycle still runs, no Worktree is passed
// (source phases will be role-gate-denied — loud, not silent), and Cleanup is
// not called for a worktree that was never created.
func TestOrchestrator_WorktreeProvisionFailure_BestEffort(t *testing.T) {
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	led := &fakeLedger{}
	runners := buildRunners(nil)
	wt := &fakeWorktree{createErr: errors.New("git worktree add failed")}
	o := NewOrchestrator(st, led, runners, WithWorktreeProvisioner(wt))

	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: "/tmp/p", GoalHash: "g"}); err != nil {
		t.Fatalf("RunCycle should not fail on best-effort worktree error: %v", err)
	}
	if len(wt.cleaned) != 0 {
		t.Errorf("Cleanup should not run when Create failed; got %v", wt.cleaned)
	}
	for _, p := range []Phase{PhaseTDD, PhaseBuild} {
		fr := runners[p].(*fakeRunner)
		if len(fr.requests) > 0 && fr.requests[0].Worktree != "" {
			t.Errorf("phase %s Worktree = %q, want empty after provision failure", p, fr.requests[0].Worktree)
		}
	}
}
