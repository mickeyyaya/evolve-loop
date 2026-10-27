//go:build integration

package core

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gateclass"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func gateFloorWorktree(t *testing.T) (*gittest.Repo, string) {
	t.Helper()
	wt := gittest.Fixture(t)
	wt.Write("go/internal/widget/widget_guard_test.go", "package widget\n\nfunc TestWidget_Guard(t *testing.T) {\n}\n")
	wt.Write("go/acs/cycle5/p_test.go", "package cycle5\n")
	wt.Git("add", "-A")
	wt.Git("commit", "-q", "-m", "base")
	return wt, strings.TrimSpace(wt.Git("rev-parse", "HEAD"))
}

func TestGateClassifierFloorChecks_FailsABuildThatSkipsAGuardTest(t *testing.T) {
	t.Parallel()
	wt, base := gateFloorWorktree(t)
	wt.Write("go/internal/widget/widget_guard_test.go", "package widget\n\nfunc TestWidget_Guard(t *testing.T) {\n\tt.Skip(\"later\")\n}\n")
	wt.Write("go/internal/policy/healing.go", "package policy\n")
	center := signalcenter.New()

	got := GateClassifierFloorChecks(context.Background(), ReviewInput{Cycle: 5, RunID: "r5", Worktree: wt.Dir, WorktreeBaseSHA: base, Signals: center})

	if len(got) != 1 || !strings.Contains(got[0], "added-skip go/internal/widget/widget_guard_test.go") {
		t.Fatalf("failures = %v, want only the added skip: the protected-surface floor reports the protected healing.go", got)
	}
	if !strings.Contains(got[0], string(gateclass.CodeI10Violation)) {
		t.Errorf("failure %q does not name the invariant code", got[0])
	}
	var signals int
	for _, e := range center.Recent() {
		if e.Code == gateclass.CodeI10Violation && e.Cycle == 5 && e.Phase == "build" {
			signals++
		}
	}
	if signals != 1 {
		t.Errorf("I-10 signals = %d, want 1", signals)
	}
}

func TestGateClassifierFloorChecks_PassesAnOrdinaryBuild(t *testing.T) {
	t.Parallel()
	wt, base := gateFloorWorktree(t)
	wt.Write("go/internal/widget/widget.go", "package widget\n")

	if got := GateClassifierFloorChecks(context.Background(), ReviewInput{Worktree: wt.Dir, WorktreeBaseSHA: base}); got != nil {
		t.Fatalf("failures = %v, want none", got)
	}
}

func TestGateClassifierFloorChecks_AGitFailureFailsTheFloor(t *testing.T) {
	t.Parallel()

	got := GateClassifierFloorChecks(context.Background(), ReviewInput{Worktree: t.TempDir()})

	if len(got) != 1 || !strings.Contains(got[0], "gate classifier") {
		t.Fatalf("failures = %v, want one gate classifier failure", got)
	}
}

func TestGateClassifierFloorChecks_FailsOpenWithoutAWorktree(t *testing.T) {
	t.Parallel()

	if got := GateClassifierFloorChecks(context.Background(), ReviewInput{}); got != nil {
		t.Fatalf("failures = %v, want none without a worktree", got)
	}
}

func TestGateClassifierFloorChecks_TheBuildMayChangeItsOwnCyclePredicates(t *testing.T) {
	t.Parallel()
	wt, base := gateFloorWorktree(t)
	wt.Write("go/acs/cycle5/p_test.go", "package cycle5\n\nconst rows = 2\n")

	if got := GateClassifierFloorChecks(context.Background(), ReviewInput{Cycle: 5, Worktree: wt.Dir, WorktreeBaseSHA: base}); got != nil {
		t.Fatalf("cycle 5 changes its own predicates: %v", got)
	}
	if got := GateClassifierFloorChecks(context.Background(), ReviewInput{Cycle: 6, Worktree: wt.Dir, WorktreeBaseSHA: base}); len(got) != 1 || !strings.Contains(got[0], "frozen-check go/acs/cycle5/p_test.go") {
		t.Fatalf("cycle 6 changes the predicates of cycle 5: %v", got)
	}
}
