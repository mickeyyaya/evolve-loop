//go:build integration

package triage

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

type canceledAfterTheFact struct{ context.Context }

func (canceledAfterTheFact) Err() error { return context.Canceled }

func committedRepo(t *testing.T, files map[string]string) *gittest.Repo {
	t.Helper()
	repo := gittest.Fixture(t)
	for rel, body := range files {
		path := filepath.Join(repo.Dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	return repo
}

func TestVanishedAtHEAD_ReportsOnlyDeclaredPathsAbsentFromTheHeadTree(t *testing.T) {
	repo := committedRepo(t, map[string]string{"go/internal/kept/k.go": "package kept\n", "README.md": "r\n"})
	if err := os.WriteFile(filepath.Join(repo.Dir, "untracked.go"), []byte("package x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	declared := []string{"go/internal/kept/k.go", "go/internal/kept/", "go/internal/gone/", "README.md", "untracked.go", "docs/missing.md"}

	gone, ok := vanishedAtHEAD(context.Background(), repo.Dir, declared)

	if want := []string{"go/internal/gone/", "untracked.go", "docs/missing.md"}; !ok || !slices.Equal(gone, want) {
		t.Fatalf("vanishedAtHEAD = %q, %v; want %q, true", gone, ok, want)
	}
}

func TestVanishedAtHEAD_NothingDeclaredIsAvailableAndEmpty(t *testing.T) {
	repo := committedRepo(t, map[string]string{"a.txt": "a\n"})

	gone, ok := vanishedAtHEAD(context.Background(), repo.Dir, nil)

	if !ok || gone != nil {
		t.Fatalf("vanishedAtHEAD = %q, %v; want nil, true", gone, ok)
	}
}

func TestVanishedAtHEAD_UnbornOrMissingRepoIsUnavailableNotEverythingVanished(t *testing.T) {
	unborn := gittest.Fixture(t)
	outside := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(outside))
	for name, root := range map[string]string{"unborn": unborn.Dir, "not a repo": outside} {
		gone, ok := vanishedAtHEAD(context.Background(), root, []string{"a.txt", "b/"})
		if ok || gone != nil {
			t.Fatalf("%s: vanishedAtHEAD = %q, %v; want nil, false", name, gone, ok)
		}
	}
}

func TestVanishedAtHEAD_ACancelSeenAfterAFailedProbeIsUnavailable(t *testing.T) {
	repo := committedRepo(t, map[string]string{"kept.txt": "k\n"})

	gone, ok := vanishedAtHEAD(canceledAfterTheFact{context.Background()}, repo.Dir, []string{"kept.txt", "gone.txt"})

	if ok || gone != nil {
		t.Fatalf("vanishedAtHEAD = %q, %v; want nil, false once the context reports cancellation", gone, ok)
	}
}

func TestCarryforwardCandidatesSection_ListsOnlyLandableBranchesOtherThanBase(t *testing.T) {
	repo := committedRepo(t, map[string]string{"base.txt": "base\n"})
	branch := func(name, file, body string) {
		repo.Git("checkout", "-q", "-b", name, "main")
		if err := os.WriteFile(filepath.Join(repo.Dir, file), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		repo.Git("add", "-A")
		repo.Git("commit", "-q", "-m", name)
		repo.Git("checkout", "-q", "main")
	}
	branch("cycle-clean", "clean.txt", "clean\n")
	branch("cycle-conflict", "base.txt", "branch side\n")
	branch("cycle-absorbed", "absorbed.txt", "absorbed\n")
	branch("feature-not-cycle", "f.txt", "f\n")
	if err := os.WriteFile(filepath.Join(repo.Dir, "base.txt"), []byte("main side\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "main moves")
	repo.Git("merge", "-q", "--no-edit", "cycle-absorbed")
	repo.Git("branch", "cycle-main", "main")

	section := CarryforwardCandidatesSection(context.Background(), repo.Dir, "cycle-main")

	if !strings.Contains(section, "- carryforward_candidates: 1 local cycle-* branch(es)") || !strings.Contains(section, "onto cycle-main") {
		t.Fatalf("section = %q, want exactly one landable candidate onto cycle-main", section)
	}
	for _, absent := range []string{"cycle-conflict", "cycle-absorbed", "feature-not-cycle", "  - cycle-main"} {
		if strings.Contains(section, absent) {
			t.Fatalf("section lists %q:\n%s", absent, section)
		}
	}
	if !strings.Contains(section, "  - cycle-clean\n") {
		t.Fatalf("section = %q, want cycle-clean listed", section)
	}
}

func TestCarryforwardCandidatesSection_NoLandableBranchIsSilent(t *testing.T) {
	repo := committedRepo(t, map[string]string{"base.txt": "base\n"})
	repo.Git("checkout", "-q", "-b", "cycle-conflict", "main")
	if err := os.WriteFile(filepath.Join(repo.Dir, "base.txt"), []byte("branch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo.Git("commit", "-q", "-am", "branch")
	repo.Git("checkout", "-q", "main")
	if err := os.WriteFile(filepath.Join(repo.Dir, "base.txt"), []byte("main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo.Git("commit", "-q", "-am", "main")

	if got := CarryforwardCandidatesSection(context.Background(), repo.Dir, "main"); got != "" {
		t.Fatalf("section = %q, want silence when nothing is landable", got)
	}
}

func TestCarryforwardCandidatesSection_NotARepositoryIsSilent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))

	if got := CarryforwardCandidatesSection(context.Background(), dir, "main"); got != "" {
		t.Fatalf("section = %q, want silence outside a repository", got)
	}
}

func TestCarryforwardCandidatesSection_ExactlyTheCapAfterSkippingBaseIsNotTruncated(t *testing.T) {
	repo := committedRepo(t, map[string]string{"base.txt": "base\n"})
	repo.Git("checkout", "-q", "-b", "side", "main")
	if err := os.WriteFile(filepath.Join(repo.Dir, "side.txt"), []byte("side\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "side")
	repo.Git("checkout", "-q", "main")
	for i := range carryforwardCandidatesMaxBranches {
		repo.Git("branch", "cycle-b"+strconv.Itoa(i), "side")
	}
	repo.Git("branch", "cycle-base", "main")

	section := CarryforwardCandidatesSection(context.Background(), repo.Dir, "cycle-base")

	want := "- carryforward_candidates: " + strconv.Itoa(carryforwardCandidatesMaxBranches) + " local cycle-* branch(es)"
	if !strings.Contains(section, want) || strings.Contains(section, "partial:") || strings.Contains(section, "  - cycle-base\n") {
		t.Fatalf("section = %q, want all %d candidates, no truncation and no base", section, carryforwardCandidatesMaxBranches)
	}
}
