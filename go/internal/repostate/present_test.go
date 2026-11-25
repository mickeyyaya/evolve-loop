package repostate

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func TestPresentFiles_ListsTrackedAndUntrackedAndSkipsDeletedAndIgnored(t *testing.T) {
	repo := gittest.Fixture(t)
	repo.Write(".gitignore", "ignored.yml\n")
	repo.Write("kept.yml", "a: 1\n")
	repo.Write("gone.yml", "a: 1\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "seed")
	if err := os.Remove(filepath.Join(repo.Dir, "gone.yml")); err != nil {
		t.Fatal(err)
	}
	repo.Write("new dir/fresh.yml", "a: 1\n")
	repo.Write("ignored.yml", "a: 1\n")

	got, err := PresentFiles(context.Background(), gitexec.Default(repo.Dir))

	want := []string{".gitignore", "kept.yml", "new dir/fresh.yml"}
	if err != nil || !slices.Equal(got, want) {
		t.Fatalf("PresentFiles = %q, %v; want %q", got, err, want)
	}
}

func TestPresentFiles_AnEmptyRepoHasNoFiles(t *testing.T) {
	repo := gittest.Fixture(t)

	got, err := PresentFiles(context.Background(), gitexec.Default(repo.Dir))

	if err != nil || len(got) != 0 {
		t.Fatalf("PresentFiles = %q, %v; want none", got, err)
	}
}

func TestPresentFiles_AGitFailureOutsideARepositoryIsAnError(t *testing.T) {
	if _, err := PresentFiles(context.Background(), gitexec.Default(t.TempDir())); err == nil {
		t.Error("PresentFiles outside a repository = nil error, want the listing failure")
	}
}

func TestPresentFiles_AFailingListingIsAnErrorFromThatListing(t *testing.T) {
	for _, failAt := range []int{1, 2} {
		repo := gittest.Fixture(t)
		repo.Write("kept.yml", "a: 1\n")
		calls := 0
		g := gitexec.Default(repo.Dir)
		real := g.Exec
		g.Exec = func(ctx context.Context, name, dir string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
			calls++
			if calls == failAt {
				return 1, nil
			}
			return real(ctx, name, dir, args, env, stdin, stdout, stderr)
		}

		got, err := PresentFiles(context.Background(), g)

		if err == nil || got != nil || calls != failAt {
			t.Errorf("listing %d failing: PresentFiles = %q, %v after %d git calls; want no files and an error at that call", failAt, got, err, calls)
		}
	}
}
