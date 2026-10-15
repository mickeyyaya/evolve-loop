package core

import (
	"context"

	"github.com/mickeyyaya/evolve-loop/go/internal/treedelta"
)

func treeDelta(ctx context.Context, git gitFn, worktree, base, tree string) ([]byte, error) {
	return treedelta.Delta(ctx, treedelta.Git(git), worktree, base, tree)
}

func identicalChange(ctx context.Context, git gitFn, worktree, base0, tree0, base1, tree1 string) (audited, composed []byte, ok bool, err error) {
	return treedelta.Identical(ctx, treedelta.Git(git), worktree, base0, tree0, base1, tree1)
}
