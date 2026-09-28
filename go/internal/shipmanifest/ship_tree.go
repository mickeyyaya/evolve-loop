package shipmanifest

import (
	"context"

	"github.com/mickeyyaya/evolve-loop/go/internal/treefence"
)

type stagedSnapshot func(ctx context.Context, worktree string, pathspec []string) (treefence.Snapshot, error)

func TakeShipTree(ctx context.Context, read GitRead, worktree, workspace string) (treefence.Snapshot, error) {
	return takeShipTree(ctx, read, worktree, workspace, treefence.TakeStaged)
}

func takeShipTree(ctx context.Context, read GitRead, worktree, workspace string, take stagedSnapshot) (treefence.Snapshot, error) {
	sel, err := Select(read, worktree, workspace)
	if err != nil {
		return treefence.Snapshot{}, err
	}
	if len(sel.Paths) == 0 {
		return take(ctx, worktree, nil)
	}
	var snap treefence.Snapshot
	_, _, err = StageRetrying(sel.Paths, func(paths []string) (string, error) {
		var takeErr error
		snap, takeErr = take(ctx, worktree, paths)
		if takeErr != nil {
			return takeErr.Error(), takeErr
		}
		return "", nil
	})
	return snap, err
}
