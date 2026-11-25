package research

import (
	"context"
	"math"
	"testing"
)

func TestScoreRecall_AmplifiedBoundaries(t *testing.T) {
	t.Parallel()
	kb := stubKB{results: map[string][]string{
		"perfect": {"a", "b"},
		"second":  {"x", "a"},
		"miss":    {"x", "y", "z"},
		"empty":   {},
		"wide":    {"a"},
	}}
	cases := []struct {
		name       string
		fm         string
		relevant   []string
		k          int
		wantRecall float64
		wantMRR    float64
	}{
		{"perfect ranker", "perfect", []string{"a", "b"}, 5, 1, 1},
		{"first relevant at rank two", "second", []string{"a"}, 5, 1, 0.5},
		{"cut at one drops rank two", "second", []string{"a"}, 1, 0, 0},
		{"all misses", "miss", []string{"a"}, 5, 0, 0},
		{"empty result list", "empty", []string{"a"}, 5, 0, 0},
		{"k larger than results", "wide", []string{"a"}, 1000, 1, 1},
		{"partial recall", "perfect", []string{"a", "q"}, 5, 0.5, 1},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			set := GoldSet{Queries: []GoldQuery{{ID: "q", Plan: "cycle-1", Query: Query{FailureMode: tc.fm}, Relevant: tc.relevant}}}
			got, err := ScoreRecall(context.Background(), kb, set, tc.k)
			if err != nil {
				t.Fatalf("ScoreRecall: %v", err)
			}
			if got.K != tc.k || got.Queries != 1 {
				t.Fatalf("K=%d Queries=%d, want %d and 1", got.K, got.Queries, tc.k)
			}
			if math.Abs(got.Recall-tc.wantRecall) > 1e-9 || math.Abs(got.MRR-tc.wantMRR) > 1e-9 {
				t.Errorf("Recall=%v MRR=%v, want %v and %v", got.Recall, got.MRR, tc.wantRecall, tc.wantMRR)
			}
		})
	}
}

func TestScoreRecall_AmplifiedRefusesNegativeKAndNoRelevant(t *testing.T) {
	t.Parallel()
	kb := stubKB{}
	good := GoldQuery{ID: "q", Plan: "cycle-1", Query: Query{FailureMode: "m"}, Relevant: []string{"a"}}
	bad := GoldQuery{ID: "r", Plan: "cycle-1", Query: Query{FailureMode: "m"}}
	if _, err := ScoreRecall(context.Background(), kb, GoldSet{Queries: []GoldQuery{good}}, -3); err == nil {
		t.Error("negative k must be refused")
	}
	if _, err := ScoreRecall(context.Background(), kb, GoldSet{Queries: []GoldQuery{good, bad}}, 5); err == nil {
		t.Error("a query with no relevant id must be refused")
	}
}
