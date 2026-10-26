package inboxcurator

import (
	"cmp"
	"path"
	"slices"
	"strings"
	"unicode"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

type Overlap struct {
	ID         string   `json:"id"`
	Similarity float64  `json:"similarity"`
	Surface    []string `json:"surface"`
}

type Thresholds struct{ Refuse, Warn float64 }

type OverlapVerdict struct{ Refused, Warned []Overlap }

const minTokenLength = 3

var stopWords = map[string]bool{
	"the": true, "and": true, "for": true, "with": true, "that": true, "this": true, "from": true, "into": true,
	"are": true, "not": true, "its": true, "was": true, "has": true, "but": true, "all": true, "any": true,
	"one": true, "when": true, "then": true, "each": true, "only": true, "must": true, "than": true, "does": true,
}

func Similarity(a, b inboxbatch.Item) float64 {
	ta, tb := textTokens(a), textTokens(b)
	union := len(ta)
	shared := 0
	for tok := range tb {
		if ta[tok] {
			shared++
		} else {
			union++
		}
	}
	if union == 0 {
		return 0
	}
	return float64(shared) / float64(union)
}

func textTokens(it inboxbatch.Item) map[string]bool {
	tokens := map[string]bool{}
	for _, text := range append([]string{it.Title}, it.Acceptance...) {
		for _, tok := range strings.FieldsFunc(strings.ToLower(text), isTokenBreak) {
			if len(tok) >= minTokenLength && !stopWords[tok] {
				tokens[tok] = true
			}
		}
	}
	return tokens
}

func isTokenBreak(r rune) bool {
	return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_'
}

func SharedSurface(a, b inboxbatch.Item) []string {
	var shared []string
	qs := b.DeclaredPaths()
	for _, p := range a.DeclaredPaths() {
		for _, q := range qs {
			if path, ok := samePlace(p, q); ok && !slices.Contains(shared, path) {
				shared = append(shared, path)
			}
		}
	}
	slices.Sort(shared)
	return shared
}

func samePlace(p, q string) (string, bool) {
	p, q = path.Clean(p), path.Clean(q)
	switch {
	case p == q:
		return p, true
	case strings.HasPrefix(q, p+"/"):
		return q, true
	case strings.HasPrefix(p, q+"/"):
		return p, true
	}
	return "", false
}

func declaresRelation(it inboxbatch.Item, id string) bool {
	return slices.Contains(it.Deps, id) || slices.Contains(it.ConnectsTo, id)
}

func JudgeOverlap(candidate inboxbatch.Item, open []inboxbatch.Item, t Thresholds) OverlapVerdict {
	var v OverlapVerdict
	for _, other := range open {
		if other.ID == candidate.ID || declaresRelation(candidate, other.ID) {
			continue
		}
		surface := SharedSurface(candidate, other)
		if len(surface) == 0 {
			continue
		}
		o := Overlap{ID: other.ID, Similarity: Similarity(candidate, other), Surface: surface}
		switch {
		case o.Similarity >= t.Refuse:
			v.Refused = append(v.Refused, o)
		case o.Similarity >= t.Warn:
			v.Warned = append(v.Warned, o)
		}
	}
	bySimilarity := func(x, y Overlap) int { return cmp.Compare(y.Similarity, x.Similarity) }
	slices.SortStableFunc(v.Refused, bySimilarity)
	slices.SortStableFunc(v.Warned, bySimilarity)
	return v
}
