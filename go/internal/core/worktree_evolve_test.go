package core

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func TestWorktreeEvolveInvocation_RunsTheWorktreesOwnEvolveFromItsSource(t *testing.T) {
	worktree := filepath.Join(t.TempDir(), "lane-7")

	got := WorktreeEvolveInvocation(worktree, "skills", "check")

	want := EvolveInvocation{
		Dir:  filepath.Join(worktree, "go"),
		Args: []string{"run", "./cmd/evolve", "skills", "check"},
		Env:  append(os.Environ(), "EVOLVE_WORKTREE_ROOT="+worktree),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("WorktreeEvolveInvocation = %+v, want %+v (the inherited environment, then the worktree key last)", got, want)
	}
}

func TestChangedWorktreePathsSinceBase_SeesCommittedWorkFromTheBase(t *testing.T) {
	repo := gittest.Fixture(t)
	writeFile(t, filepath.Join(repo.Dir, "base.txt"), "base\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-qm", "base")
	base := repo.Git("rev-parse", "HEAD")
	writeFile(t, filepath.Join(repo.Dir, "go", "internal", "skillcheck", "commands.go"), "package skillcheck\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-qm", "lane")
	writeFile(t, filepath.Join(repo.Dir, "untracked.md"), "new\n")
	cases := []struct {
		name string
		base string
		want []string
	}{
		{"from the cycle base", base, []string{"go/internal/skillcheck/commands.go", "untracked.md"}},
		{"without a base it diffs HEAD", "", []string{"untracked.md"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ChangedWorktreePathsSinceBase(context.Background(), repo.Dir, tc.base)

			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ChangedWorktreePathsSinceBase = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestChangedFloorPaths_NoWorktreeListsNothing(t *testing.T) {
	if got := changedFloorPaths(context.Background(), ReviewInput{WorktreeBaseSHA: "abc123"}); got != nil {
		t.Errorf("changedFloorPaths without a worktree = %q, want nil (never diff the caller's directory)", got)
	}
}
