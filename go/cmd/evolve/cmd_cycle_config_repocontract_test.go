package main

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
)

func TestRepoContractPackSeam_DefaultsToShipsOwnPack(t *testing.T) {
	t.Parallel()
	want := reflect.ValueOf(ship.RunRepoContractPack).Pointer()
	if got := reflect.ValueOf(repoContractPack).Pointer(); got != want {
		t.Fatal("the build floor must run ship's own repo-contract pack, so the suite list has one home")
	}
}

func TestProductionBuildFloorChecks_CorrectsARedRepoContractPackByTheTestsName(t *testing.T) {
	repo := gittest.Fixture(t)
	repo.Write("go/go.mod", "module github.com/mickeyyaya/evolve-loop/go\n\ngo 1.23\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	wt := repo.Dir
	const red = "github.com/mickeyyaya/evolve-loop/go/internal/rawgitratchet.TestRatchet_NoNewRawGitFixtures"
	var ranIn []string
	prev := repoContractPack
	t.Cleanup(func() { repoContractPack = prev })
	repoContractPack = func(_ context.Context, _, root string, _ []string) ([]string, string, error) {
		ranIn = append(ranIn, root)
		return []string{red}, "", errors.New("exit status 1")
	}
	got := core.BuildHandoffFloor{productionFloorCheck()}.Failures(context.Background(), core.ReviewInput{Phase: string(core.PhaseBuild), Worktree: wt})
	if len(ranIn) != 1 || ranIn[0] != wt {
		t.Fatalf("the production floor runs the repo-contract pack once, handed the build worktree; ran in %v", ranIn)
	}
	if !strings.Contains(strings.Join(got, "\n"), "TestRatchet_NoNewRawGitFixtures") {
		t.Fatalf("the production floor names the pack's red test; got %v", got)
	}
}
