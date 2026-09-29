package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
)

func TestRepoContractPackSeam_DefaultsToShipsOwnPack(t *testing.T) {
	want := reflect.ValueOf(ship.RunRepoContractPack).Pointer()
	if got := reflect.ValueOf(repoContractPack).Pointer(); got != want {
		t.Fatal("the build floor must run ship's own repo-contract pack, so the suite list has one home")
	}
}

func TestProductionBuildFloorChecks_CorrectsARedRepoContractPackByTheTestsName(t *testing.T) {
	wt := t.TempDir()
	if err := os.MkdirAll(filepath.Join(wt, "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	const red = "github.com/mickeyyaya/evolve-loop/go/internal/rawgitratchet.TestRatchet_NoNewRawGitFixtures"
	var ranIn []string
	prev := repoContractPack
	t.Cleanup(func() { repoContractPack = prev })
	repoContractPack = func(_ context.Context, moduleDir string) ([]string, error) {
		ranIn = append(ranIn, moduleDir)
		return []string{red}, errors.New("exit status 1")
	}
	got := productionBuildFloorChecks(context.Background(), core.ReviewInput{Phase: string(core.PhaseBuild), Worktree: wt})
	if len(ranIn) != 1 || ranIn[0] != filepath.Join(wt, "go") {
		t.Fatalf("the production floor runs the repo-contract pack once in the worktree's module; ran in %v", ranIn)
	}
	if !strings.Contains(strings.Join(got, "\n"), "TestRatchet_NoNewRawGitFixtures") {
		t.Fatalf("the production floor names the pack's red test; got %v", got)
	}
}
