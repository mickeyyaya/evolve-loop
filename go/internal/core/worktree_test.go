package core

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestGitWorktree_CreateUsesNamedBranch validates the production provisioner
// against real git: the worktree must be on a NAMED branch (cycle-<N>), not a
// detached HEAD — worktree-aware ship resolves the branch via symbolic-ref and
// ff-merges it, so a detached worktree would break every cycle's ship.
func TestGitWorktree_CreateUsesNamedBranch(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")
	if err := os.WriteFile(filepath.Join(root, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "-q", "-m", "init")

	g := gitWorktree{}
	wt, err := g.Create(root, 77)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer g.Cleanup(root, wt)

	out, err := exec.Command("git", "-C", wt, "symbolic-ref", "--short", "HEAD").Output()
	if err != nil {
		t.Fatalf("worktree is detached (symbolic-ref failed): %v — ship would break", err)
	}
	if got := strings.TrimSpace(string(out)); got != "cycle-77" {
		t.Fatalf("worktree branch = %q, want cycle-77", got)
	}

	// Idempotent reuse returns the same valid worktree.
	if wt2, err := g.Create(root, 77); err != nil || wt2 != wt {
		t.Fatalf("reuse: got (%q, %v), want (%q, nil)", wt2, err, wt)
	}

	g.Cleanup(root, wt)
	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Fatalf("worktree not removed after Cleanup: stat err=%v", err)
	}
}

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

	// Source phases run with cwd=worktree.
	for _, p := range []Phase{PhaseTDD, PhaseBuild} {
		fr := runners[p].(*fakeRunner)
		if len(fr.requests) == 0 {
			t.Fatalf("phase %s never ran", p)
		}
		if got := fr.requests[0].Worktree; got != "/tmp/wt/cycle-10" {
			t.Errorf("phase %s Worktree = %q, want /tmp/wt/cycle-10", p, got)
		}
	}
	// Read-mostly phases stay on the main tree (empty Worktree).
	for _, p := range []Phase{PhaseIntent, PhaseScout, PhaseTriage, PhaseAudit} {
		fr := runners[p].(*fakeRunner)
		if len(fr.requests) > 0 && fr.requests[0].Worktree != "" {
			t.Errorf("phase %s Worktree = %q, want empty (main tree)", p, fr.requests[0].Worktree)
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
