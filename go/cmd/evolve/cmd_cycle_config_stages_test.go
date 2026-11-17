package main

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func newStagedProductionFixture(t *testing.T) (*gittest.Repo, core.ReviewInput, *int) {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Write("go/go.mod", "module github.com/mickeyyaya/evolve-loop/go\n\ngo 1.23\n")
	repo.Write("go/internal/core/cyclerun.go", "package core\n")
	repo.Write("go/internal/widget/widget_guard_test.go", "package widget\n\nfunc TestWidget_Guard(t *testing.T) {\n}\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	base := repo.Git("rev-parse", "HEAD")
	repo.Write("go/p/a.go", "package p\n\nfunc F() {}\n")
	packRuns := 0
	prevPack, prevLint := repoContractPack, lintPackages
	t.Cleanup(func() { repoContractPack, lintPackages = prevPack, prevLint })
	repoContractPack = func(context.Context, string, string, []string) ([]string, string, error) {
		packRuns++
		return nil, "", nil
	}
	return repo, core.ReviewInput{Phase: string(core.PhaseBuild), Worktree: repo.Dir, WorktreeBaseSHA: base}, &packRuns
}

func TestProductionFloor_AStaticRedSkipsTheRepoContractPackAndReportsEveryStaticFailure(t *testing.T) {
	repo, in, packRuns := newStagedProductionFixture(t)
	repo.Write("go/internal/core/cyclerun.go", "package core\n\nvar changed = 1\n")
	repo.Write("go/internal/widget/widget_guard_test.go", "package widget\n\nfunc TestWidget_Guard(t *testing.T) {\n\tt.Skip(\"later\")\n}\n")
	lintPackages = func(context.Context, string, []string) (string, error) {
		return "p/a.go:3:2: Error return value is not checked (errcheck)", nil
	}

	got := strings.Join(core.BuildHandoffFloor{productionFloorCheck()}.Failures(context.Background(), in), "\n")

	if !strings.Contains(got, "protected control-plane path changed: go/internal/core/cyclerun.go") || !strings.Contains(got, "p/a.go:3:2") || !strings.Contains(got, "added-skip go/internal/widget/widget_guard_test.go") {
		t.Fatalf("the protected-surface, gate-classifier and lint failures of one round report together; got %q", got)
	}
	if *packRuns != 0 {
		t.Fatalf("a static red skips the repo-contract pack; it ran %d time(s)", *packRuns)
	}
}

func TestProductionFloor_AGreenStaticStageRunsTheRepoContractPack(t *testing.T) {
	_, in, packRuns := newStagedProductionFixture(t)
	lintPackages = func(context.Context, string, []string) (string, error) { return "", nil }

	got := core.BuildHandoffFloor{productionFloorCheck()}.Failures(context.Background(), in)

	if *packRuns != 1 {
		t.Fatalf("a green static stage runs the repo-contract pack once; it ran %d time(s), failures %v", *packRuns, got)
	}
}
