package integration

import "context"

type PlaneSyncFunc func(ctx context.Context) (SyncOutcome, error)

func (f PlaneSyncFunc) Sync(ctx context.Context) (SyncOutcome, error) { return f(ctx) }

func PlaneOf(repo Repo) PlaneSync {
	return PlaneSyncFunc(func(ctx context.Context) (SyncOutcome, error) { return SyncToOrigin(ctx, repo) })
}
