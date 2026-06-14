//go:build integration

// resume_normalize_test.go (integration tier — uses real git via gitInRepo).
// RED contract for the resume.go:278 TODO
// (cycle-156 parity): RunCycle soft-resets a committing builder's worktree
// commits back to the cycle base after PhaseBuild (normalizeWorktreeToBase)
// so audit + binding see PENDING changes; the crash-resume path
// (RunCycleFromPhase) lacked that normalize because the base SHA lived in a
// RunCycle-local variable. Fix: persist CycleState.WorktreeBaseSHA at
// worktree creation and run one shared post-build normalize in both loops.
//
// The persisted-base design introduces one risk the local variable never
// had: after a rebase recovery (soak 2026-06-12) the stored base may no
// longer be an ancestor of the worktree HEAD, and `reset --soft` to a
// non-ancestor would repoint the branch and stage a huge spurious diff.
// normalizeWorktreeToBase must therefore verify ancestry before resetting.
package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// addCommit writes a file and commits it in dir, returning the new HEAD.
func addCommit(t *testing.T, dir, name, msg string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(name+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitInRepo(t, dir, "add", "-A")
	gitInRepo(t, dir, "commit", "-q", "-m", msg)
	return gitInRepo(t, dir, "rev-parse", "HEAD")
}

// TestRunCycleFromPhase_NormalizesBuildWorktree: resuming FROM PhaseBuild
// with a committing builder must re-expose the builder's commit as pending
// changes (cycle-156 Option C), exactly like RunCycle does. The base SHA
// comes from the persisted CycleState.WorktreeBaseSHA.
func TestRunCycleFromPhase_NormalizesBuildWorktree(t *testing.T) {
	t.Parallel()
	repo, ws := initBindingRepo(t, "cycle-9")
	// The per-cycle worktree: base commit + a builder commit on top.
	wt, base := newRepoWithBaseCommit(t)
	builderHead := addCommit(t, wt, "feature.go", "feat: x [worktree-build]")

	st := &fakeStorage{
		state: State{LastCycleNumber: 9},
		cycleState: CycleState{
			CycleID:         9,
			WorkspacePath:   ws,
			ActiveWorktree:  wt,
			WorktreeBaseSHA: base,
		},
	}
	led := &fakeLedger{}
	o := NewOrchestrator(st, led, buildRunners(nil))
	if _, err := o.RunCycleFromPhase(context.Background(), CycleRequest{
		ProjectRoot: repo,
	}, &ResumePoint{Phase: string(PhaseBuild), CycleID: 9}); err != nil {
		t.Fatalf("RunCycleFromPhase: %v", err)
	}

	if head := gitInRepo(t, wt, "rev-parse", "HEAD"); head != base {
		t.Fatalf("RED: worktree HEAD=%s, want base=%s — resume-from-build did not "+
			"normalize the committing builder's worktree (was %s)", head, base, builderHead)
	}
	diff := gitInRepo(t, wt, "diff", "HEAD", "--name-only")
	if !strings.Contains(diff, "feature.go") {
		t.Fatalf("feature.go must be pending for audit after the resume normalize; "+
			"git diff HEAD --name-only=%q", diff)
	}
}

// TestRunCycleFromPhase_NoNormalizeWhenResumingPastBuild: resuming from
// AUDIT replays no build phase, so the worktree must stay untouched (the
// operator's manual recovery may have left a deliberate committed state).
func TestRunCycleFromPhase_NoNormalizeWhenResumingPastBuild(t *testing.T) {
	t.Parallel()
	repo, ws := initBindingRepo(t, "cycle-10")
	wt, base := newRepoWithBaseCommit(t)
	committed := addCommit(t, wt, "feature.go", "feat: x [worktree-build]")

	st := &fakeStorage{
		state: State{LastCycleNumber: 10},
		cycleState: CycleState{
			CycleID:         10,
			WorkspacePath:   ws,
			ActiveWorktree:  wt,
			WorktreeBaseSHA: base,
		},
	}
	o := NewOrchestrator(st, &fakeLedger{}, buildRunners(nil))
	if _, err := o.RunCycleFromPhase(context.Background(), CycleRequest{
		ProjectRoot: repo,
	}, &ResumePoint{Phase: string(PhaseAudit), CycleID: 10}); err != nil {
		t.Fatalf("RunCycleFromPhase: %v", err)
	}

	if head := gitInRepo(t, wt, "rev-parse", "HEAD"); head != committed {
		t.Errorf("resume-from-audit must not touch the worktree; HEAD=%s, want %s", head, committed)
	}
}

// TestNormalizeWorktreeToBase_SkipsNonAncestorBase: after a rebase recovery
// the persisted base may not be an ancestor of the worktree HEAD. Resetting
// to a non-ancestor would stage a spurious mega-diff; the helper must skip
// (best-effort WARN) and leave HEAD untouched.
func TestNormalizeWorktreeToBase_SkipsNonAncestorBase(t *testing.T) {
	t.Parallel()
	dir, base := newRepoWithBaseCommit(t)
	// Side branch from base — sideSHA is NOT an ancestor of the main line.
	gitInRepo(t, dir, "checkout", "-q", "-b", "side")
	side := addCommit(t, dir, "side.go", "side commit")
	gitInRepo(t, dir, "checkout", "-q", "-")
	mainHead := addCommit(t, dir, "main.go", "main commit")
	if base == side || side == mainHead {
		t.Fatal("precondition: three distinct commits required")
	}

	normalizeWorktreeToBase(context.Background(), dir, side)

	if head := gitInRepo(t, dir, "rev-parse", "HEAD"); head != mainHead {
		t.Fatalf("RED: HEAD=%s, want %s — normalize reset to a NON-ANCESTOR base "+
			"(rebase-recovery hazard: branch repointed + spurious staged diff)", head, mainHead)
	}
}

// TestNormalizeWorktreeToBase_Idempotent: running the normalize twice is a
// no-op the second time (HEAD already at base) and never loses the pending
// work the first run exposed.
func TestNormalizeWorktreeToBase_Idempotent(t *testing.T) {
	t.Parallel()
	dir, base := newRepoWithBaseCommit(t)
	addCommit(t, dir, "feature.go", "feat: x [worktree-build]")

	normalizeWorktreeToBase(context.Background(), dir, base)
	normalizeWorktreeToBase(context.Background(), dir, base)

	if head := gitInRepo(t, dir, "rev-parse", "HEAD"); head != base {
		t.Fatalf("HEAD=%s, want base=%s after double normalize", head, base)
	}
	if diff := gitInRepo(t, dir, "diff", "HEAD", "--name-only"); !strings.Contains(diff, "feature.go") {
		t.Fatalf("feature.go must survive a double normalize; got %q", diff)
	}
}

// TestRunCycle_PersistsWorktreeBaseSHA: RunCycle must persist the worktree
// HEAD-at-creation into CycleState.WorktreeBaseSHA (not a run-local
// variable) so a crash-resume can normalize. The fake provisioner hands
// RunCycle a real git repo so rev-parse yields a real SHA.
func TestRunCycle_PersistsWorktreeBaseSHA(t *testing.T) {
	t.Parallel()
	repo, _ := initBindingRepo(t, "cycle-11")
	wt, base := newRepoWithBaseCommit(t)

	st := &fakeStorage{state: State{LastCycleNumber: 10}} // cycle 11
	o := NewOrchestrator(st, &fakeLedger{}, buildRunners(nil),
		WithWorktreeProvisioner(&fakeWorktree{path: wt}))
	if _, err := o.RunCycle(context.Background(), CycleRequest{
		ProjectRoot: repo, GoalHash: "g",
	}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}

	if st.cycleState.WorktreeBaseSHA != base {
		t.Fatalf("RED: persisted WorktreeBaseSHA=%q, want %q — the cycle base "+
			"must live in CycleState so the resume path can normalize",
			st.cycleState.WorktreeBaseSHA, base)
	}
}
