package commitgate

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/launchrecord"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/treestate"
)

const reviewRunPrefix = "run:"

func isReviewRun(token string) bool {
	return strings.HasPrefix(stripWhitespace(token), reviewRunPrefix)
}

func (o Options) resolveReviewRuns(ctx context.Context, res *Result) (Options, int) {
	tokens := strings.Split(o.Reviewers, ",")
	if !slices.ContainsFunc(tokens, isReviewRun) {
		return o, ExitPass
	}
	tree, err := treestate.SHA(ctx, o.Runner, o.RepoRoot, o.Env)
	if err != nil {
		res.log("cannot compute the tree SHA that the review runs must match: %v", err)
		return o, ExitGitFatal
	}
	names := make([]string, 0, len(tokens))
	for _, token := range tokens {
		id, isRun := strings.CutPrefix(stripWhitespace(token), reviewRunPrefix)
		if !isRun {
			names = append(names, token)
			continue
		}
		run, refused := o.reviewRun(id, tree)
		if refused != "" {
			res.log("DENY: review run %s does not attest this commit: %s", id, refused)
			return o, ExitFail
		}
		names = append(names, run.Profile)
		res.reviewRuns = append(res.reviewRuns, run)
	}
	o.Reviewers = strings.Join(names, ",")
	return o, ExitPass
}

func (o Options) reviewRun(id, tree string) (ReviewRun, string) {
	rec, err := launchrecord.Read(paths.EvolveDirOf(o.RepoRoot), id)
	switch {
	case err != nil:
		return ReviewRun{}, err.Error()
	case rec.CLI == "":
		return ReviewRun{}, "the record has no bridge launch"
	case rec.WritesSource:
		return ReviewRun{}, "the run is a writer, not a review"
	case rec.Verdict != core.VerdictPASS && rec.Verdict != core.VerdictWARN:
		return ReviewRun{}, fmt.Sprintf("the review gave the verdict %q, not PASS or WARN", rec.Verdict)
	case rec.ArtifactSHA256 == "":
		return ReviewRun{}, "the review wrote no report"
	case !rec.BoundTo(tree):
		return ReviewRun{}, "the review did not see the staged tree (the tree changed during or after the review)"
	}
	return ReviewRun{RunID: rec.RunID, Profile: rec.Profile, CLI: rec.CLI, Family: rec.Family, Model: rec.Model}, ""
}
