//go:build integration

package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func addCommit(t *testing.T, dir, name, msg string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(name+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitInRepo(t, dir, "add", "-A")
	gitInRepo(t, dir, "commit", "-q", "-m", msg)
	return gitInRepo(t, dir, "rev-parse", "HEAD")
}

func TestRunCycleFromPhase_NormalizesBuildWorktree(t *testing.T) {
	t.Parallel()
	repo, ws := initBindingRepo(t, "cycle-9")
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
	// A resumed cycle that completes normally prunes its worktree at exit
	// (cycle_worktree_teardown.go); the fake provisioner records that disposal
	// instead of performing it, so this test can still read the worktree's git
	// state after the call returns. What the teardown itself does is pinned by
	// resume_worktree_teardown_test.go, not here.
	o := NewOrchestrator(st, led, buildRunners(nil), WithWorktreeProvisioner(&fakeWorktree{path: wt}))
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

// Resuming from AUDIT replays no build phase, so the worktree must stay
// untouched: the operator's manual recovery may have left a deliberate
// committed state.
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
	// See the sibling test: the exit teardown is recorded, not performed, so
	// this test can still read the worktree it is asserting about.
	o := NewOrchestrator(st, &fakeLedger{}, buildRunners(nil), WithWorktreeProvisioner(&fakeWorktree{path: wt}))
	if _, err := o.RunCycleFromPhase(context.Background(), CycleRequest{
		ProjectRoot: repo,
	}, &ResumePoint{Phase: string(PhaseAudit), CycleID: 10}); err != nil {
		t.Fatalf("RunCycleFromPhase: %v", err)
	}

	if head := gitInRepo(t, wt, "rev-parse", "HEAD"); head != committed {
		t.Errorf("resume-from-audit must not touch the worktree; HEAD=%s, want %s", head, committed)
	}
}

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
