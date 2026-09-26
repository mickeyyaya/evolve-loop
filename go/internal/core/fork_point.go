package core

import "context"

// forkPoint is the main commit the lane's HEAD builds on. Unlike main itself, it stays put when a peer
// lands after the rebase.
func forkPoint(ctx context.Context, git gitFn, worktree string) (string, error) {
	return gitStdout(ctx, git, worktree, "merge-base", "HEAD", "main")
}
