package research

import (
	"context"
	"sort"
)

type FileKB struct {
	roots  []string
	recall int
}

func NewFileKB(roots []string) *FileKB { return NewFileKBWithRecall(roots, maxResults) }

func NewFileKBWithRecall(roots []string, recallK int) *FileKB {
	if recallK <= 0 {
		recallK = maxResults
	}
	return &FileKB{roots: roots, recall: recallK}
}

const maxResults = 5

type scored struct {
	lesson Lesson
	score  float64
}

func (k *FileKB) Lookup(_ context.Context, q Query) ([]Lesson, error) {
	terms := q.terms()
	var ranked []scored
	seen := map[string]struct{}{}
	for _, root := range k.roots {
		for _, file := range listLessonFiles(root) {
			lessons, err := parseLessonFile(file)
			if err != nil {
				continue
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
		conf = defaultConfidence
	}
	return raw * conf
}

const (
	stepMatchWeight   = 2.0
	overlapWeight     = 1.0
	defaultConfidence = 0.5
)

func tokenSet(s string) map[string]struct{} {
	set := map[string]struct{}{}
	for _, t := range tokenize(s) {
		set[t] = struct{}{}
	}
	return set
}
