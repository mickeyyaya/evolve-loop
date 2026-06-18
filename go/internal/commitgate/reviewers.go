package commitgate

import "strings"

// reviewersSatisfied enforces the --reviewers precondition: the declared
// reviewers must cover a SIMPLIFY capability AND a REVIEW capability (general
// code-reviewer OR a reviewer matching one of the changed languages). It is the
// Go mirror of the bash cap-satisfied check.
//
// Matching is capability-based and ECC-aware: each declared reviewer is
// normalized by stripping any namespace prefix (everything up to and including
// the last ':', so `ecc:go-reviewer` -> `go-reviewer`) and removing ALL
// whitespace (matching the bash `tr -d '[:space:]'`). A capability is satisfied
// if ANY of its synonyms appears among the normalized reviewers.
//
// Only ONE of {code-reviewer, <lang>-reviewer} is required; the language
// reviewer is the richer choice but the general one suffices.
// code-review-simplify satisfies both simplify and review at once.
//
// On failure it appends the same DENY diagnostics the bash runner logs and
// returns false.
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

// normalizeReviewers splits the --reviewers CSV and normalizes each entry:
// strip the namespace prefix (text up to and including the last ':') and remove
// all whitespace. Empty results are dropped. The returned set is used only for
// the precondition check — the attestation records the raw spellings.
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

// capSatisfied reports whether any synonym is present in the normalized set.
func capSatisfied(set map[string]bool, synonyms []string) bool {
	for _, s := range synonyms {
		if set[s] {
			return true
		}
	}
	return false
}

// stripWhitespace removes ASCII whitespace characters, mirroring the bash
// `tr -d '[:space:]'`.
func stripWhitespace(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', '\n', '\r', '\v', '\f':
			return -1
		}
		return r
	}, s)
}
