package research

import (
	"context"
	"reflect"
	"testing"
)

func rankedIDsFromRoots(t *testing.T, roots []string, q Query) []string {
	t.Helper()
	got, err := NewFileKB(roots).Lookup(context.Background(), q)
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	return ids(got)
}

func TestFileKB_EqualScoreRanksHigherConfidenceFirst(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeLesson(t, dir, "a.yaml", `- id: inst-A
  pattern: p
  description: "unrelated words"
  confidence: 0.5
  failureContext: {failedStep: audit}
`)
	writeLesson(t, dir, "b.yaml", `- id: inst-B
  pattern: p
  description: "predicate here"
  confidence: 1.0
  failureContext: {failedStep: build}
`)

	got := rankedIDsFromRoots(t, []string{dir}, Query{Source: "audit", Keywords: []string{"predicate"}})

	if want := []string{"inst-B", "inst-A"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v: equal scores must rank the higher confidence first, not the lower ID", got, want)
	}
}

func TestFileKB_EqualScoreAndConfidenceRanksByID(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeLesson(t, dir, "a.yaml", `- id: inst-B
  pattern: p
  description: "predicate"
  confidence: 0.7
`)
	writeLesson(t, dir, "b.yaml", `- id: inst-A
  pattern: p
  description: "predicate"
  confidence: 0.7
`)

	got := rankedIDsFromRoots(t, []string{dir}, Query{Keywords: []string{"predicate"}})

	if want := []string{"inst-A", "inst-B"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v: a full tie must break by ID, not by file order", got, want)
	}
}

func TestScore_UnscoredLessonCountsAtDefaultConfidence(t *testing.T) {
	t.Parallel()
	unscored := Lesson{FailedStep: "audit"}
	scored := Lesson{FailedStep: "audit", Confidence: 1.0}
	q := Query{Source: "audit"}

	if got, want := score(unscored, q, nil), 1.0; got != want {
		t.Fatalf("unscored lesson score = %v, want %v", got, want)
	}
	if score(unscored, q, nil) >= score(scored, q, nil) {
		t.Fatalf("an unscored lesson must rank below a fully confident one with the same match")
	}
}
