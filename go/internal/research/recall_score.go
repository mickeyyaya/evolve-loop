package research

import (
	"context"
	"errors"
	"fmt"
)

type RecallScore struct {
	K       int     `json:"k"`
	Queries int     `json:"queries"`
	Recall  float64 `json:"recall"`
	MRR     float64 `json:"mrr"`
}

var (
	errNilKB            = errors.New("research: kb is nil")
	errNonPositiveK     = errors.New("research: k must be positive")
	errEmptyGoldSet     = errors.New("research: gold set has no queries")
	errNoRelevantLabels = errors.New("research: query has no relevant entries")
)

func ScoreRecall(ctx context.Context, kb KB, set GoldSet, k int) (RecallScore, error) {
	if kb == nil {
		return RecallScore{}, errNilKB
	}
	if k <= 0 {
		return RecallScore{}, errNonPositiveK
	}
	if len(set.Queries) == 0 {
		return RecallScore{}, errEmptyGoldSet
	}

	var totalRecall float64
	var totalRR float64
	for _, q := range set.Queries {
		relevant := uniqueNonEmpty(q.Relevant)
		if len(relevant) == 0 {
			return RecallScore{}, fmt.Errorf("query %q: %w", q.ID, errNoRelevantLabels)
		}
		got, err := kb.Lookup(ctx, q.Query)
		if err != nil {
			return RecallScore{}, fmt.Errorf("lookup query %q: %w", q.ID, err)
		}
		hits, firstRank := scoreQuery(got, relevant, k)
		totalRecall += float64(hits) / float64(len(relevant))
		if firstRank > 0 {
			totalRR += 1.0 / float64(firstRank)
		}
	}

	n := float64(len(set.Queries))
	return RecallScore{
		K:       k,
		Queries: len(set.Queries),
		Recall:  totalRecall / n,
		MRR:     totalRR / n,
	}, nil
}

func uniqueNonEmpty(items []string) map[string]struct{} {
	set := make(map[string]struct{}, len(items))
	for _, item := range items {
		if item != "" {
			set[item] = struct{}{}
		}
	}
	return set
}

func scoreQuery(got []Lesson, relevant map[string]struct{}, k int) (hits int, firstRank int) {
	seen := make(map[string]struct{}, min(len(got), k))
	for i := 0; i < len(got) && i < k; i++ {
		id := got[i].ID
		if id == "" {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		if _, ok := relevant[id]; ok {
			hits++
			if firstRank == 0 {
				firstRank = i + 1
			}
		}
	}
	return hits, firstRank
}
