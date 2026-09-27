package core

import "context"

func forkPoint(ctx context.Context, git gitFn, worktree string) (string, error) {
	return gitStdout(ctx, git, worktree, "merge-base", "HEAD", "main")
}
