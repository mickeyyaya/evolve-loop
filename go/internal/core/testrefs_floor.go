package core

import (
	"context"
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/codequality"
	"github.com/mickeyyaya/evolve-loop/go/internal/commentaudit"
	"github.com/mickeyyaya/evolve-loop/go/internal/testrefs"
)

func testrefsFloorFailures(ctx context.Context, in ReviewInput) []string {
	if in.Worktree == "" {
		return nil
	}
	base := floorBase(in)
	prefix, err := moduleRelPrefix(in.Worktree, codequality.ModuleDir(in.Worktree))
	var refusal string
	if err == nil {
		refusal, err = testrefs.CheckAgainstBase(prefix, commentaudit.ReadAtBase(worktreeGit(ctx, in.Worktree), base), worktreeFile(in.Worktree))
	}
	if err != nil {
		fmt.Fprintf(in.errOut(), "[build-floor] WARN named-test floor could not judge the change against %s (%v); the repo-contract pack still runs the guard\n", base, err)
		return nil
	}
	if refusal == "" {
		return nil
	}
	return []string{fmt.Sprintf("named-test baseline against the base %s: a baseline only shrinks. Remove each added row, then write the test the doc names or fix the name in the doc; do not edit %s to add a row:\n  %s", base, testrefs.BaselineRelPath, refusal)}
}
