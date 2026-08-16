package research

import (
	"context"
	"sort"
)

// FileKB is the filesystem-backed KB: it reads lesson YAML from a set of roots
// and ranks matches deterministically. No caching (the corpus is small and a
// lookup happens at most once per cycle); no LLM (ranking is pure arithmetic).
type FileKB struct {
	roots  []string
	recall int
}

// NewFileKB builds a FileKB over the given lesson-directory roots (e.g. from
// SearchPathsFromEnv) at the built-in recall bound. Roots that don't exist are
// simply skipped at lookup time.
func NewFileKB(roots []string) *FileKB { return NewFileKBWithRecall(roots, maxResults) }

// NewFileKBWithRecall builds a FileKB whose Lookup returns at most recallK
// lessons — the top-k PREFIX of the same deterministic ranking, never a
// resample. recallK ≤ 0 falls back to the built-in maxResults: a zero-recall KB
// would silently disable the advisor's recall memory, which is a degradation an
// operator typo must not be able to cause. The caller (the composition root)
// resolves recallK from policy.ResearchConfig().RecallK.
func NewFileKBWithRecall(roots []string, recallK int) *FileKB {
	if recallK <= 0 {
		recallK = maxResults
	}
	return &FileKB{roots: roots, recall: recallK}
}

// maxResults is the built-in recall bound — enough for the advisor's
// recall section without flooding the prompt. Operator-overridable via the
// policy.json "research" block; see NewFileKBWithRecall.
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
	// A zero-value FileKB (struct literal rather than a constructor) must still
	// recall: an unset bound means "built-in", never "none".
	recall := k.recall
	if recall <= 0 {
		recall = maxResults
	}
	out := make([]Lesson, 0, min(len(ranked), recall))
	for i := 0; i < len(ranked) && i < recall; i++ {
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
