//go:build acs

// Package cycle2474 encodes the acceptance of backfill-a-path-handling-defects-lane.
package cycle2474

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/repostate"
)

func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		target := filepath.Join(root, rel)
		err := os.MkdirAll(filepath.Dir(target), 0o755)
		if err == nil {
			err = os.WriteFile(target, []byte(body), 0o644)
		}
		if err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
}

func seededRepo(t *testing.T, files map[string]string) *gittest.Repo {
	t.Helper()
	repo := gittest.Fixture(t)
	writeTree(t, repo.Dir, files)
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "seed")
	return repo
}

func readTree(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("read %q: %v", rel, err)
	}
	return string(data)
}

func quarantine(t *testing.T, repo *gittest.Repo) {
	t.Helper()
	stashed, err := core.QuarantineDirtyTree(context.Background(), repo.Dir, "acs-cycle-2474")
	if err != nil {
		t.Fatalf("QuarantineDirtyTree: %v", err)
	}
	if !stashed {
		t.Fatal("QuarantineDirtyTree reported stashed=false for a dirty source tree")
	}
}

func TestC2474_001_NonASCIIEditedTrackedFileIsStashedAndTreeIsClean(t *testing.T) {
	repo := seededRepo(t, map[string]string{"go/é.go": "package a\n"})
	writeTree(t, repo.Dir, map[string]string{"go/é.go": "package a\n\nvar leaked = 1\n"})

	quarantine(t, repo)

	if status := repo.Git("status", "--porcelain"); status != "" {
		t.Fatalf("tree not clean after quarantine: %q", status)
	}
	repo.Git("stash", "pop")
	if got := readTree(t, repo.Dir, "go/é.go"); got != "package a\n\nvar leaked = 1\n" {
		t.Fatalf("stash lost the edit; recovered %q", got)
	}
}

func TestC2474_002_NonASCIIUntrackedFileIsStashedAndTreeIsClean(t *testing.T) {
	repo := seededRepo(t, map[string]string{"go/base.go": "package a\n"})
	writeTree(t, repo.Dir, map[string]string{"go/ü new.go": "package a\n"})

	quarantine(t, repo)

	if status := repo.Git("status", "--porcelain"); status != "" {
		t.Fatalf("tree not clean after quarantine: %q", status)
	}
	repo.Git("stash", "pop", "--index")
	if got := readTree(t, repo.Dir, "go/ü new.go"); got != "package a\n" {
		t.Fatalf("stash lost the untracked file; recovered %q", got)
	}
}

func TestC2474_003_NonASCIILoopManagedPathStaysDirtyBesideAStashedNonASCIISource(t *testing.T) {
	repo := seededRepo(t, map[string]string{
		"go/é.go":               "package a\n",
		"knowledge-base/é.md":   "k\n",
		".evolve/runs/ü/run.md": "r\n",
	})
	writeTree(t, repo.Dir, map[string]string{
		"go/é.go":               "package a\n\nvar leaked = 1\n",
		"knowledge-base/é.md":   "k2\n",
		".evolve/runs/ü/run.md": "r2\n",
	})

	quarantine(t, repo)

	dirty := repo.Git("-c", "core.quotePath=false", "diff", "--name-only")
	want := ".evolve/runs/ü/run.md\nknowledge-base/é.md"
	if dirty != want {
		t.Fatalf("dirty paths after quarantine = %q, want only the loop-managed paths: %q", dirty, want)
	}
}

func TestC2474_004_DirtyFileNameWithEdgeSpacesIsStashed(t *testing.T) {
	for _, name := range []string{"go/ lead.go", "go/trail.go ", "go/with space.go"} {
		t.Run(name, func(t *testing.T) {
			repo := seededRepo(t, map[string]string{name: "package a\n"})
			writeTree(t, repo.Dir, map[string]string{name: "package a\n\nvar leaked = 1\n"})

			quarantine(t, repo)

			if status := repo.Git("status", "--porcelain"); status != "" {
				t.Fatalf("tree not clean after quarantine: %q", status)
			}
			repo.Git("stash", "pop")
			if got := readTree(t, repo.Dir, name); got != "package a\n\nvar leaked = 1\n" {
				t.Fatalf("stash lost the edit; recovered %q", got)
			}
		})
	}
}

var awkwardNames = []string{"d/ lead.go", "d/trail.go ", "d/é.go", "d/new\nline.go", "d/plain.go"}

func awkwardNameFiles() map[string]string {
	files := make(map[string]string, len(awkwardNames))
	for _, name := range awkwardNames {
		files[name] = "x\n"
	}
	return files
}

func TestC2474_005_TrackedTreeReturnsEveryNameExactlyAsGitStoresIt(t *testing.T) {
	repo := seededRepo(t, awkwardNameFiles())

	got, err := repostate.TrackedTree(repo.Dir, "d")

	if err != nil {
		t.Fatalf("TrackedTree: %v", err)
	}
	slices.Sort(got)
	want := slices.Clone(awkwardNames)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("TrackedTree = %q, want %q", got, want)
	}
}

func TestC2474_006_TrackedFilesKeepsEdgeSpaceAndNonASCIINamesAndSkipsNestedOnes(t *testing.T) {
	files := awkwardNameFiles()
	files["d/sub/ nested.go"] = "x\n"
	repo := seededRepo(t, files)

	got, err := repostate.TrackedFiles(repo.Dir, "d")

	if err != nil {
		t.Fatalf("TrackedFiles: %v", err)
	}
	slices.Sort(got)
	want := slices.Clone(awkwardNames)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("TrackedFiles = %q, want %q", got, want)
	}
}

func TestC2474_007_TrackedSetKeysKeepLeadingSpaceAndNonASCIIStems(t *testing.T) {
	repo := seededRepo(t, awkwardNameFiles())

	set, err := repostate.TrackedSet(repo.Dir, "d", ".go")

	if err != nil {
		t.Fatalf("TrackedSet: %v", err)
	}
	for _, stem := range []string{" lead", "é", "plain"} {
		if !set[stem] {
			t.Errorf("TrackedSet lacks stem %q; got %v", stem, set)
		}
	}
	if set["lead"] {
		t.Errorf("TrackedSet holds the trimmed stem %q that no tracked file has", "lead")
	}
}
