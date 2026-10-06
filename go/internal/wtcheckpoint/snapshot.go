package wtcheckpoint

import (
	"context"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

type trees struct{ staged, full string }

func snapshot(ctx context.Context, tree gitexec.Git) (trees, error) {
	staged, err := tree.TreeOfIndexCopy(ctx)
	if err != nil {
		return trees{}, err
	}
	full, err := tree.TreeOfIndexCopy(ctx, append([]string{"add", "-A", "--", "."}, snapshotExclusions...))
	if err != nil {
		return trees{}, err
	}
	return trees{staged: staged, full: full}, nil
}
