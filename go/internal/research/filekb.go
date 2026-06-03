package research

import (
	"context"
	"sort"
)

// FileKB is the filesystem-backed KB: it reads lesson YAML from a set of roots
// and ranks matches deterministically. No caching (the corpus is small and a
// lookup happens at most once per cycle); no LLM (ranking is pure arithmetic).
type FileKB struct {
	roots []string
}

// NewFileKB builds a FileKB over the given lesson-directory roots (e.g. from
// SearchPathsFromEnv). Roots that don't exist are simply skipped at lookup time.
func NewFileKB(roots []string) *FileKB { return &FileKB{roots: roots} }

// maxResults bounds how many lessons a Lookup returns — enough for the advisor's
// recall section without flooding the prompt.
const maxResults = 5

// scored pairs a lesson with its relevance score for ranking.
type scored struct {
	lesson Lesson
	score  float64
}

// Lookup loads every lesson under the roots, scores each against the query, and
// returns the best matches (score > 0) ranked best-first. Deterministic: ties
// break by higher confidence, then by ID, so the same corpus+query always yields
// the same order. A malformed lesson file is skipped (best-effort recall must not
// fail the cycle), not fatal.
func (k *FileKB) Lookup(_ context.Context, q Query) ([]Lesson, error) {
	terms := q.terms()
	var ranked []scored
	seen := map[string]struct{}{} // dedupe by canonical Lesson.ID across roots
	for _, root := range k.roots {
		for _, file := range listLessonFiles(root) {
			lessons, err := parseLessonFile(file)
			if err != nil {
				continue // skip rot; recall is best-effort
			}
			for _, l := range lessons {
				if l.ID != "" {
					if _, dup := seen[l.ID]; dup {
						continue
					}
					seen[l.ID] = struct{}{}
				}
				if s := score(l, q, terms); s > 0 {
					ranked = append(ranked, scored{lesson: l, score: s})
				}
			}
		}
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		if ranked[i].lesson.Confidence != ranked[j].lesson.Confidence {
			return ranked[i].lesson.Confidence > ranked[j].lesson.Confidence
		}
		return ranked[i].lesson.ID < ranked[j].lesson.ID
	})
	out := make([]Lesson, 0, min(len(ranked), maxResults))
	for i := 0; i < len(ranked) && i < maxResults; i++ {
		out = append(out, ranked[i].lesson)
	}
	return out, nil
}

// score computes a lesson's relevance to a query: a strong signal for an exact
// source/step match, plus token overlap between the query terms and the lesson's
// searchable text, all weighted by the lesson's confidence (a high-confidence
// lesson outranks a low-confidence one with equal textual overlap). Pure.
func score(l Lesson, q Query, terms []string) float64 {
	var raw float64
	if q.Source != "" && q.Source == l.FailedStep {
		raw += stepMatchWeight
	}
	if len(terms) > 0 {
		hay := tokenSet(l.Pattern + " " + l.Description + " " + l.PreventiveAction + " " + l.ErrorCategory)
		for _, t := range terms {
			if _, ok := hay[t]; ok {
				raw += overlapWeight
			}
		}
	}
	if raw == 0 {
		return 0
	}
	conf := l.Confidence
	if conf <= 0 {
		conf = defaultConfidence // an unscored lesson still counts, just weakly
	}
	return raw * conf
}

const (
	stepMatchWeight   = 2.0
	overlapWeight     = 1.0
	defaultConfidence = 0.5
)

// tokenSet returns the set of tokens in s for O(1) membership in scoring.
func tokenSet(s string) map[string]struct{} {
	set := map[string]struct{}{}
	for _, t := range tokenize(s) {
		set[t] = struct{}{}
	}
	return set
}
