package core

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/builddiff"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

func writeBuildDiffSummary(ctx context.Context, worktree, base, workspace string, warn io.Writer) {
	path, body, ok := buildReportFile(workspace)
	if !ok {
		return
	}
	summary, err := builddiff.Derive(ctx, gitexec.Git{Dir: worktree, Exec: gitRunner}, base)
	next, warnings := builddiff.Apply(body, summary, base, err)
	for _, w := range warnings {
		fmt.Fprintf(warn, "[orchestrator] WARN %s\n", w)
	}
	if werr := os.WriteFile(path, []byte(next), 0o644); werr != nil {
		fmt.Fprintf(warn, "[orchestrator] WARN build-diff-summary: write %s: %v\n", path, werr)
	}
}
