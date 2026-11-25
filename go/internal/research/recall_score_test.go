package research

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"
)

type stubKB struct {
	results map[string][]string
	err     error
}

func (s stubKB) Lookup(_ context.Context, q Query) ([]Lesson, error) {
	if s.err != nil {
		return nil, s.err
	}
	ids := s.results[q.FailureMode]
	out := make([]Lesson, len(ids))
	for i, id := range ids {
		out[i] = Lesson{ID: id}
	}
	return out, nil
}

func TestScoreRecall_MetricsAndCutoff(t *testing.T) {
	kb := stubKB{results: map[string][]string{
		"hit1": {"a", "b"},
		"hit3": {"x", "y", "c"},
		"deep": {"x1", "x2", "x3", "x4", "x5", "d"},
		"dup":  {"a", "", "a", "b"},
	}}
	set := GoldSet{Queries: []GoldQuery{
		{ID: "q1", Plan: "cycle-1", Query: Query{FailureMode: "hit1"}, Relevant: []string{"a"}},
		{ID: "q2", Plan: "cycle-2", Query: Query{FailureMode: "hit3"}, Relevant: []string{"c", "z"}},
		{ID: "q3", Plan: "cycle-3", Query: Query{FailureMode: "deep"}, Relevant: []string{"d"}},
		{ID: "q4", Plan: "cycle-4", Query: Query{FailureMode: "dup"}, Relevant: []string{"a", "b"}},
	}}
	got, err := ScoreRecall(context.Background(), kb, set, 5)
	if err != nil {
		t.Fatalf("ScoreRecall: %v", err)
	}
	wantRecall := (1.0 + 0.5 + 0.0 + 1.0) / 4.0
	wantMRR := (1.0 + 1.0/3.0 + 0.0 + 1.0) / 4.0
	if got.K != 5 || got.Queries != 4 {
		t.Fatalf("K=%d Queries=%d, want 5 and 4", got.K, got.Queries)
	}
	if math.Abs(got.Recall-wantRecall) > 1e-6 || math.Abs(got.MRR-wantMRR) > 1e-6 {
		t.Errorf("got Recall=%v MRR=%v, want %v and %v", got.Recall, got.MRR, wantRecall, wantMRR)
	}
}

func TestScoreRecall_Refusals(t *testing.T) {
	okKB := stubKB{results: map[string][]string{"m": {"a"}}}
	validSet := GoldSet{Queries: []GoldQuery{{ID: "q1", Plan: "cycle-1", Query: Query{FailureMode: "m"}, Relevant: []string{"a"}}}}
	sentinelErr := errors.New("boom")

	cases := []struct {
		name string
		kb   KB
		set  GoldSet
		k    int
		want error
	}{
		{"nil kb", nil, validSet, 5, errNilKB},
		{"zero k", okKB, validSet, 0, errNonPositiveK},
		{"negative k", okKB, validSet, -3, errNonPositiveK},
		{"empty set", okKB, GoldSet{}, 5, errEmptyGoldSet},
		{"no relevant ids", okKB, GoldSet{Queries: []GoldQuery{{ID: "q1", Query: Query{FailureMode: "m"}}}}, 5, errNoRelevantLabels},
		{"empty relevant id only", okKB, GoldSet{Queries: []GoldQuery{{ID: "q1", Query: Query{FailureMode: "m"}, Relevant: []string{""}}}}, 5, errNoRelevantLabels},
		{"ranker error", stubKB{err: sentinelErr}, validSet, 5, sentinelErr},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ScoreRecall(context.Background(), tc.kb, tc.set, tc.k)
			if !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want errors.Is(_, %v)", err, tc.want)
			}
		})
	}
}

func TestLoadGoldSet_RoundTripAndErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gold.json")
	want := GoldSet{Queries: []GoldQuery{{
		ID:       "q-1",
		Plan:     "cycle-1200",
		Query:    Query{Source: "build", FailureMode: "gate", Consequence: "block", Keywords: []string{"worktree"}},
		Relevant: []string{"lesson-1"},
	}}}
	raw, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadGoldSet(path)
	if err != nil {
		t.Fatalf("LoadGoldSet: %v", err)
	}
	if len(got.Queries) != 1 || got.Queries[0].ID != "q-1" || got.Queries[0].Query.Source != "build" {
		t.Errorf("unexpected loaded gold set: %+v", got)
	}
	if _, err := LoadGoldSet(filepath.Join(dir, "missing.json")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("missing file err = %v, want os.ErrNotExist", err)
	}
	corrupt := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(corrupt, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadGoldSet(corrupt); err == nil {
		t.Errorf("corrupt JSON should error")
	}
}
