package core

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/builddiff"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

type buildDiffInputs struct {
	Worktree, Base, Workspace string
	Regenerated               []string
}

func writeBuildDiffSummary(ctx context.Context, in buildDiffInputs, warn io.Writer) {
	path, body, ok := buildReportFile(in.Workspace)
	if !ok {
		return
	}
	summary, err := builddiff.Derive(ctx, gitexec.Git{Dir: in.Worktree, Exec: gitRunner}, in.Base)
	summary.Regenerated = in.Regenerated
	next, warnings := builddiff.Apply(body, summary, in.Base, err)
	for _, w := range warnings {
		fmt.Fprintf(warn, "[orchestrator] WARN %s\n", w)
	}
	if werr := os.WriteFile(path, []byte(next), 0o644); werr != nil {
		fmt.Fprintf(warn, "[orchestrator] WARN build-diff-summary: write %s: %v\n", path, werr)
	}
}
