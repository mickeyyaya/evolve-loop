package integration

import (
	"context"
	"fmt"
	"strings"
)

type SyncOutcome string

const (
	SyncCurrent  SyncOutcome = "current"
	SyncMoved    SyncOutcome = "moved"
	SyncAhead    SyncOutcome = "ahead"
	SyncDiverged SyncOutcome = "diverged"
	SyncBlocked  SyncOutcome = "blocked"
)

func SyncToOrigin(ctx context.Context, r Repo) (SyncOutcome, error) {
	if _, err := r.Git(ctx, r.Dir, "", "fetch", "origin", mainBranch); err != nil {
		return "", fmt.Errorf("integration: fetch origin %s: %w", mainBranch, err)
	}
	local, err := r.revParse(ctx, branchRef(mainBranch))
	if err != nil {
		return "", err
	}
	remote, err := r.revParse(ctx, "refs/remotes/origin/"+mainBranch)
	if err != nil {
		return "", err
	}
	if local == remote {
		return SyncCurrent, nil
	}
	ahead, err := r.IsAncestor(ctx, remote, local)
	if err != nil {
		return "", err
	}
	if ahead {
		return SyncAhead, nil
	}
	behind, err := r.IsAncestor(ctx, local, remote)
	if err != nil {
		return "", err
	}
	if !behind {
		return SyncDiverged, nil
	}
	if _, err := r.Git(ctx, r.Dir, "", "merge", "--ff-only", "origin/"+mainBranch); err != nil {
		return SyncBlocked, nil
	}
	return SyncMoved, nil
}

func (r Repo) revParse(ctx context.Context, ref string) (string, error) {
	out, err := r.Git(ctx, r.Dir, "", "rev-parse", "--verify", ref)
	if err != nil {
		return "", fmt.Errorf("integration: rev-parse %s: %w", ref, err)
	}
	sha := strings.TrimSpace(out)
	if err := fullObjectID(sha); err != nil {
		return "", err
	}
	return sha, nil
}
