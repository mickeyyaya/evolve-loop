package research

import (
	"context"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// Query is the taxonomy-shaped lookup key. It mirrors core.Taxonomy
// (Source/FailureMode/Consequence) plus free Keywords, so the orchestrator can
// translate a failed phase's VerdictReason straight into a Query without the KB
// importing core (dependency stays one-directional).
type Query struct {
	Source      string   // matches Lesson.FailedStep (e.g. "audit","build")
	FailureMode string   // free-text failure mode; matched as keywords
	Consequence string   // the classification string; matched as keywords
	Keywords    []string // extra terms (e.g. defect slugs)
}

// terms returns the lowercased, de-duplicated set of search terms a query
// contributes. Pure; deterministic order (input order, first occurrence wins).
func (q Query) terms() []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(s string) {
		for _, tok := range tokenize(s) {
			if _, ok := seen[tok]; ok {
				continue
			}
			seen[tok] = struct{}{}
			out = append(out, tok)
		}
	}
	add(q.FailureMode)
	add(q.Consequence)
	for _, k := range q.Keywords {
		add(k)
	}
	return out
}

// KB is the read port over the lessons corpus. Small by design (one method) so
// the orchestrator can depend on the interface and tests can fake it.
type KB interface {
	// Lookup returns lessons relevant to q, ranked best-first. An empty result
	// (no corpus, or nothing matched) is NOT an error — it is the signal that
	// the failure is novel (the WS2 consolidation trigger).
	Lookup(ctx context.Context, q Query) ([]Lesson, error)
}

// SplitSearchPaths splits a colon-separated EVOLVE_KB_SEARCH_PATHS value into
// roots, dropping empties. Exposed so the composition root can build a FileKB
// from the env var without duplicating the parse.
func SplitSearchPaths(raw string) []string {
	var roots []string
	for _, p := range strings.Split(raw, ":") {
		if p = strings.TrimSpace(p); p != "" {
			roots = append(roots, p)
		}
	}
	return roots
}

// SearchPathsFromEnv returns KB search paths from the policy config,
// falling back to the documented default when cfg.KBSearchPaths is empty.
// Replaced EVOLVE_KB_SEARCH_PATHS env read (cycle-17).
func SearchPathsFromEnv(cfg policy.PathsConfig) []string {
	if strings.TrimSpace(cfg.KBSearchPaths) != "" {
		return SplitSearchPaths(cfg.KBSearchPaths)
	}
	return SplitSearchPaths(defaultSearchPaths)
}

const defaultSearchPaths = "knowledge-base/research/:.evolve/instincts/lessons/:docs/research/"

// tokenize lowercases s and splits on non-alphanumeric runs, dropping tokens
// shorter than 3 chars (noise). Pure.
func tokenize(s string) []string {
	var out []string
	var b strings.Builder
	flush := func() {
		if b.Len() >= 3 {
			out = append(out, b.String())
		}
		b.Reset()
	}
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	return out
}
