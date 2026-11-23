package gc

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func TestAmpRestoreArchive_RefusesCyclesWithNoLeafAndCreatesNothing(t *testing.T) {
	for _, cycle := range []int{-1, 0, 1, 999999} {
		repo := gittest.Fixture(t)
		o := WorktreeOptions{
			ProjectRoot:  repo.Dir,
			WorktreeBase: filepath.Join(repo.Dir, ".evolve", "worktrees"),
			EvolveDir:    filepath.Join(repo.Dir, ".evolve"),
		}
		before := repo.Git("branch", "--list")
		_, err := RestoreArchive(o, cycle, filepath.Join(t.TempDir(), "wt"))
		if err == nil {
			t.Fatalf("cycle %d: want an error for a cycle with no leaf", cycle)
		}
		if after := repo.Git("branch", "--list"); after != before {
			t.Fatalf("cycle %d: branches changed: %q -> %q", cycle, before, after)
		}
	}
}
