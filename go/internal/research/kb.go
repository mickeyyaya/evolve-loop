package research

import (
	"context"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type Query struct {
	Source      string
	FailureMode string
	Consequence string
	Keywords    []string
}

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

type KB interface {
	Lookup(ctx context.Context, q Query) ([]Lesson, error)
}

func SplitSearchPaths(raw string) []string {
	var roots []string
	for _, p := range strings.Split(raw, ":") {
		if p = strings.TrimSpace(p); p != "" {
			roots = append(roots, p)
		}
	}
	return roots
}

func SearchPathsFromEnv(cfg policy.PathsConfig) []string {
	if strings.TrimSpace(cfg.KBSearchPaths) != "" {
		return SplitSearchPaths(cfg.KBSearchPaths)
	}
	return SplitSearchPaths(defaultSearchPaths)
}

const defaultSearchPaths = "knowledge-base/research/:.evolve/instincts/lessons/:docs/research/"

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
