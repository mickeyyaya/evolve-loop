package core

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func scopeJourneyRepo(t *testing.T, existing, files map[string]string) (ReviewInput, *strings.Builder) {
	t.Helper()
	repo := gittest.Fixture(t)
	writeFile(t, filepath.Join(repo.Dir, "go", "go.mod"), "module example.com/m\n\ngo 1.23\n")
	for name, body := range existing {
		writeFile(t, filepath.Join(repo.Dir, "go", name), body)
	}
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	base := repo.Git("rev-parse", "HEAD")
	for name, body := range files {
		writeFile(t, filepath.Join(repo.Dir, "go", name), body)
	}
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "build")
	var stderr strings.Builder
	return ReviewInput{Phase: string(PhaseBuild), Worktree: repo.Dir, ProjectRoot: t.TempDir(), WorktreeBaseSHA: base, Signals: signalcenter.New(), Stderr: &stderr}, &stderr
}

const (
	scopeRealSource  = "package real\n\nfunc R() int { return 1 }\n"
	scopeRealCovered = "package real\n\nimport \"testing\"\n\nfunc TestR(t *testing.T) { _ = R() }\n"
	scopeBridgeSrc   = "package bridge\n\nfunc Boot() int { return 1 }\n"
)

func TestDefaultBuildFloorChecks_ARealRedStaysRedBesideAnEnvExclusivePackage(t *testing.T) {
	t.Parallel()
	in, stderr := scopeJourneyRepo(t, map[string]string{
		"real/r.go":                  scopeRealSource,
		"real/r_test.go":             scopeRealCovered,
		"real/r_integration_test.go": "//go:build integration\n\npackage real\n\nimport \"testing\"\n\nfunc TestRedUnderTag(t *testing.T) { t.Fatal(\"red under the tag\") }\n",
	}, map[string]string{
		"internal/bridge/b.go":      scopeBridgeSrc,
		"internal/bridge/b_test.go": "package bridge\n\nimport \"testing\"\n\nfunc TestAlwaysRed(t *testing.T) { _ = Boot(); t.Fatal(\"red\") }\n",
		"real/r.go":                 strings.Replace(scopeRealSource, "1", "2", 1),
	})
	failures := strings.Join(DefaultBuildFloorChecks(context.Background(), in), "\n")

	if !strings.Contains(failures, "TestRedUnderTag") {
		t.Fatalf("failures = %q, want the real red of ./real named: a real red must never print GREEN", failures)
	}
	if strings.Contains(failures, "TestAlwaysRed") || strings.Contains(stderr.String(), "./internal/bridge was RED") {
		t.Errorf("failures = %q, stderr = %q: internal/bridge is env-exclusive and must not run at any floor entry", failures, stderr.String())
	}
}

func TestDefaultBuildFloorChecks_AGreenEnvExclusiveChangeIsNotJudgedByCoverage(t *testing.T) {
	t.Parallel()
	in, _ := scopeJourneyRepo(t, nil, map[string]string{
		"internal/bridge/b.go": scopeBridgeSrc,
		"real/r.go":            scopeRealSource,
		"real/r_test.go":       scopeRealCovered,
	})

	failures := strings.Join(DefaultBuildFloorChecks(context.Background(), in), "\n")

	if strings.Contains(failures, "internal/bridge") {
		t.Fatalf("failures = %q, want no coverage refusal for a package that no floor run covers", failures)
	}
}

func TestFloorTestPackages_OwnsTheScopeOfEveryFloorRun(t *testing.T) {
	t.Parallel()
	var warn strings.Builder
	paths := []string{"go/internal/bridge/b.go", "go/q/q.go", "go/acs/x/x.go"}

	got := floorTestPackages(paths, &warn)

	if strings.Join(got, " ") != "./q" || !strings.Contains(warn.String(), "./internal/bridge") {
		t.Fatalf("packages = %v, warn = %q, want only ./q and a NOTE for the env-exclusive package", got, warn.String())
	}
}
