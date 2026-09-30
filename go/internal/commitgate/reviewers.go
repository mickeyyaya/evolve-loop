package commitgate

import "strings"

func (o Options) reviewersSatisfied(langs []string, res *Result) bool {
	norm := normalizeReviewers(o.Reviewers)

	simplifySyn := []string{"code-simplifier", "code-review-simplify", "refactor"}
	reviewSyn := []string{"code-reviewer", "code-review", "code-review-simplify"}
	for _, l := range langs {
		switch l {
		case "go":
			reviewSyn = append(reviewSyn, "go-reviewer", "go-review")
		case "python":
			reviewSyn = append(reviewSyn, "python-reviewer", "python-review")
		case "ts", "js":
			reviewSyn = append(reviewSyn, "typescript-reviewer", "typescript-review")
		case "rust":
			reviewSyn = append(reviewSyn, "rust-reviewer", "rust-review")
		}
	}

	var missing []string
	if !capSatisfied(norm, simplifySyn) {
		missing = append(missing, "simplify")
	}
	if !capSatisfied(norm, reviewSyn) {
		missing = append(missing, "review")
	}
	if len(missing) == 0 {
		return true
	}
	res.log("DENY: missing required review capability: %s", strings.Join(missing, " "))
	res.log("  simplify ← code-simplifier | code-review-simplify | refactor")
	res.log("  review   ← code-reviewer | code-review | a matching <lang>-reviewer (ECC variants OK)")
	res.log("run them, then pass --reviewers (use the /commit skill).")
	return false
}

func normalizeReviewers(csv string) map[string]bool {
	set := map[string]bool{}
	for _, r := range strings.Split(csv, ",") {
		if i := strings.LastIndex(r, ":"); i >= 0 {
			r = r[i+1:]
		}
		r = stripWhitespace(r)
		if r != "" {
			set[r] = true
		}
	}
	return set
}

func capSatisfied(set map[string]bool, synonyms []string) bool {
	for _, s := range synonyms {
		if set[s] {
			return true
		}
	}
	return false
}

func stripWhitespace(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', '\n', '\r', '\v', '\f':
			return -1
		}
		return r
	}, s)
}
