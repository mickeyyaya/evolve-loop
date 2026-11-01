package main

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func TestProductionBuildFloorChecks_TheComposedFloorRunsTheGateClassifier(t *testing.T) {
	wt := gittest.Fixture(t)
	wt.Write("go/internal/widget/widget_guard_test.go", "package widget\n\nfunc TestWidget_Guard(t *testing.T) {\n}\n")
	wt.Git("add", "-A")
	wt.Git("commit", "-q", "-m", "base")
	base := wt.Git("rev-parse", "HEAD")
	wt.Write("go/internal/widget/widget_guard_test.go", "package widget\n\nfunc TestWidget_Guard(t *testing.T) {\n\tt.Skip(\"later\")\n}\n")
	prev := repoContractPack
	t.Cleanup(func() { repoContractPack = prev })
	repoContractPack = func(context.Context, string, string) ([]string, string, error) { return nil, "", nil }

	got := productionBuildFloorChecks(context.Background(), core.ReviewInput{
		Phase: string(core.PhaseBuild), Cycle: 7, Worktree: wt.Dir, WorktreeBaseSHA: base, ProjectRoot: t.TempDir(),
	})

	if !slices.ContainsFunc(got, func(f string) bool { return strings.Contains(f, "added-skip go/internal/widget/widget_guard_test.go") }) {
		t.Fatalf("the composed floor must fail the added skip through the gate classifier: %v", got)
	}
}
