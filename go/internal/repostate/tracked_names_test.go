package repostate

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func TestTrackedTree_ReturnsEveryNameExactlyAsGitStoresIt(t *testing.T) {
	repo := gittest.Fixture(t)
	names := []string{"d/ lead.go", "d/trail.go ", "d/é.go", "d/plain.go"}
	for _, name := range names {
		path := filepath.Join(repo.Dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "seed")

	got, err := TrackedTree(repo.Dir, "d")

	if err != nil {
		t.Fatalf("TrackedTree: %v", err)
	}
	slices.Sort(got)
	want := slices.Clone(names)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("TrackedTree = %q, want %q", got, want)
	}
}
