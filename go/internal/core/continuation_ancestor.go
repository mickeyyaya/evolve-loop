package core

import (
	"context"
	"fmt"
	"regexp"

	"github.com/mickeyyaya/evolve-loop/go/internal/continuation"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

const CtxKeyContinuationAncestry = "continuation_ancestry"

var salvageSHAPattern = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)

func continuationAncestry(ctx context.Context, projectGit gitexec.Git, workspace string) string {
	c, ok, err := continuation.ReadManifest(workspace)
	if err != nil || !ok {
		return ""
	}
	verdict := "unknown"
	if salvageSHAPattern.MatchString(c.SnapshotSHA) {
		verdict = ancestorVerdict(ctx, projectGit, c.SnapshotSHA)
	}
	return fmt.Sprintf("continuation of cycle-%d: salvage %s ancestor-of-HEAD=%s", c.Cycle, c.SnapshotSHA, verdict)
}

func ancestorVerdict(ctx context.Context, projectGit gitexec.Git, sha string) string {
	_, _, code, err := projectGit.Capture(ctx, "merge-base", "--is-ancestor", sha, "HEAD")
	switch {
	case err != nil:
		return "unknown"
	case code == 0:
		return "true"
	case code == 1:
		return "false"
	default:
		return "unknown"
	}
}
