package commitgate

import (
	"context"
	"strings"
)

var docsReviewSyn = []string{"docs-review"}

func isDocsOnly(files []string) bool {
	if len(files) == 0 {
		return false
	}
	for _, f := range files {
		if !strings.HasSuffix(f, ".md") {
			return false
		}
	}
	return true
}

func (o Options) requiredReviewSatisfied(files, langs []string, res *Result) bool {
	if !isDocsOnly(files) {
		return o.reviewersSatisfied(langs, res)
	}
	if capSatisfied(normalizeReviewers(o.Reviewers), docsReviewSyn) {
		return true
	}
	res.log("DENY: missing required review capability: docs review")
	res.log("  docs review ← docs-review (every changed path is Markdown)")
	res.log("run it, then pass --reviewers (use the /commit skill).")
	return false
}

func (o Options) listedChange(ctx context.Context) []string {
	paths, err := o.changeListing(ctx)
	if err != nil {
		return nil
	}
	return paths
}
