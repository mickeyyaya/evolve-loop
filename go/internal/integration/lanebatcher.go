package integration

import (
	"context"
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type Batcher interface {
	For(b PendingBatch) func(ctx context.Context, recs []DeliverableRecord) (Composition, error)
	Missing() []string
}

type LaneBatcher struct {
	Repo     Repo
	Composer Composer
	Wave     string
}

func (l LaneBatcher) Missing() []string {
	if l.Composer.Gate == nil {
		return []string{"Gate"}
	}
	return nil
}

func (l LaneBatcher) For(pb PendingBatch) func(ctx context.Context, recs []DeliverableRecord) (Composition, error) {
	composer := l.Composer
	composer.Stage = policy.FleetIntegrationEnforce
	return func(ctx context.Context, recs []DeliverableRecord) (Composition, error) {
		b, err := l.cut(ctx, pb)
		if err != nil {
			return Composition{}, err
		}
		return composer.composeAll(ctx, b, recs)
	}
}

func (l LaneBatcher) cut(ctx context.Context, pb PendingBatch) (Batch, error) {
	if err := deleteBranchRef(ctx, l.Repo, branchRef(PoolBranch(pb.Pool, pb.Index))); err != nil {
		return Batch{}, err
	}
	if _, err := l.Repo.Git(ctx, l.Repo.Dir, "", "fetch", "--quiet", "--no-tags", gitexec.Remote, gitexec.OriginBranchRefspec(mainBranch)); err != nil {
		return Batch{}, fmt.Errorf("integration: fetch origin %s: %w", mainBranch, err)
	}
	return l.Repo.CutFromOrigin(ctx, Batch{Wave: l.Wave, Pool: pb.Pool, Index: pb.Index})
}
