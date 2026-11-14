package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/commitgate"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func TestLintPackagesSeam_DefaultsToTheCommitGatesRunner(t *testing.T) {
	t.Parallel()
	want := reflect.ValueOf(commitgate.LintPackages).Pointer()
	if got := reflect.ValueOf(lintPackages).Pointer(); got != want {
		t.Fatal("the build floor must run the commit gate's golangci-lint runner, so the runner has one home")
	}
}

func TestProductionBuildFloorChecks_CorrectsALintFindingByFileAndLine(t *testing.T) {
	repo := gittest.Fixture(t)
	write := func(rel, body string) {
		full := filepath.Join(repo.Dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		writeFileOrFail(t, full, body)
	}
	write("go/go.mod", "module example.com/m\n\ngo 1.23\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	base := repo.Git("rev-parse", "HEAD")
	write("go/p/a.go", "package p\n\nfunc F() {}\n")
	var ran []string
	prev := lintPackages
	t.Cleanup(func() { lintPackages = prev })
	lintPackages = func(_ context.Context, moduleDir string, pkgs []string) (string, error) {
		ran = append(ran, moduleDir+" "+strings.Join(pkgs, ","))
		return "p/a.go:3:2: Error return value is not checked (errcheck)", nil
	}

	got := productionBuildFloorChecks(context.Background(), core.ReviewInput{Phase: string(core.PhaseBuild), Worktree: repo.Dir, WorktreeBaseSHA: base})

	if want := filepath.Join(repo.Dir, "go") + " ./p"; len(ran) != 1 || ran[0] != want {
		t.Fatalf("lint ran %v, want once as %q", ran, want)
	}
	if !strings.Contains(strings.Join(got, "\n"), "p/a.go:3:2") {
		t.Fatalf("the production floor names the lint finding; got %v", got)
	}
}
