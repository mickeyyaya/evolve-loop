package main

import (
	"context"
	"errors"
	"io/fs"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/derived"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

var productionDerivedRunner = derivedRunner

func TestProductionDerivedRunner_RunsTheEvolveVerbInTheWorktree(t *testing.T) {
	t.Parallel()
	missing := filepath.Join(t.TempDir(), "no-such-worktree")

	err := productionDerivedRunner(missing)(context.Background(), "docs", "index", "check")

	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("err = %v, want the go run failure of the missing worktree", err)
	}
}

func TestProductionBuildFloorChecks_CorrectsAStaleDerivedFileByItsGenerateVerb(t *testing.T) {
	repo := gittest.Fixture(t)
	repo.Write("go/go.mod", "module example.com/m\n\ngo 1.23\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	base := repo.Git("rev-parse", "HEAD")
	repo.Write("go/cmd/newpkg/main.go", "package main\n\nfunc main() {}\n")
	prev, prevLint := derivedRunner, lintPackages
	t.Cleanup(func() { derivedRunner, lintPackages = prev, prevLint })
	lintPackages = func(context.Context, string, []string) (string, error) { return "", nil }
	var worktrees []string
	derivedRunner = func(worktree string) derived.Generator {
		worktrees = append(worktrees, worktree)
		return func(_ context.Context, args ...string) error {
			if strings.Join(args, " ") != "ci darwin-set check" {
				return nil
			}
			return exec.Command("sh", "-c", "exit 3").Run()
		}
	}

	got := core.BuildHandoffFloor{productionFloorCheck()}.Failures(context.Background(), core.ReviewInput{Phase: string(core.PhaseBuild), Worktree: repo.Dir, WorktreeBaseSHA: base})

	if !strings.Contains(strings.Join(got, "\n"), "evolve ci darwin-set generate") {
		t.Fatalf("the production floor must name the darwin-set generate verb; got %v", got)
	}
	if len(worktrees) == 0 || worktrees[0] != repo.Dir {
		t.Fatalf("derived runner bound to %v, want the lane worktree %s", worktrees, repo.Dir)
	}
}
