package commitgate

import (
	"context"
	"strings"
)

var docsReviewSyn = []string{"docs-review"}

func DocsOnly(files []string) bool {
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
	if !DocsOnly(files) {
		return o.reviewersSatisfied(langs, res)
	}
	if len(MissingReviewCapabilities(files, langs, strings.Split(o.Reviewers, ","))) == 0 {
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
