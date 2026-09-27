package core

import (
	"context"

	"github.com/mickeyyaya/evolve-loop/go/internal/treedelta"
)

// treeDelta is the byte-exact change from base to tree (treedelta.Delta over the git seam).
func treeDelta(ctx context.Context, git gitFn, worktree, base, tree string) ([]byte, error) {
	return treedelta.Delta(ctx, treedelta.Git(git), worktree, base, tree)
}

// identicalChange holds when the change from base1 to tree1 is, byte for byte, the audited change from
// base0 to tree0, and not empty (treedelta.Identical over the git seam).
func identicalChange(ctx context.Context, git gitFn, worktree, base0, tree0, base1, tree1 string) (audited, composed []byte, ok bool, err error) {
	return treedelta.Identical(ctx, treedelta.Git(git), worktree, base0, tree0, base1, tree1)
}
