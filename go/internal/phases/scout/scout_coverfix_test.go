package scout

import (
	"reflect"
	"testing"
)

func TestCoverfixMergeDigestsRanksByOccurrencesThenConfidenceAndSortsCrossSignals(t *testing.T) {
	solo := CandidateTask{Slug: "solo", Files: []string{"a.go"}, Confidence: 0.9}
	weaker := CandidateTask{Slug: "weaker", Files: []string{"c.go"}, Confidence: 0.5}
	twice := CandidateTask{Slug: "twice", Files: []string{"b.go"}, Confidence: 0.2}
	twiceConfident := twice
	twiceConfident.Confidence = 0.7
	digests := []ScanDigest{
		{SliceID: "s1", CandidateTasks: []CandidateTask{weaker, solo, twice}, CrossSliceSignals: []CrossSignal{{Pattern: "zeta"}, {Pattern: "alpha"}}},
		{SliceID: "s2", CandidateTasks: []CandidateTask{twiceConfident}, CrossSliceSignals: []CrossSignal{{Pattern: "zeta"}, {Pattern: "alpha"}}},
	}
	got := mergeDigests(digests)

	if len(got.Tasks) != 3 || got.Tasks[0].Slug != "twice" || got.Tasks[0].Occurrences != 2 || got.Tasks[1].Slug != "solo" || got.Tasks[2].Slug != "weaker" {
		t.Errorf("a task seen in two slices must outrank a more confident single sighting, ties broken by confidence, got %+v", got.Tasks)
	}
	if got.Tasks[0].Confidence != 0.7 {
		t.Errorf("a merged task keeps its highest confidence, got %v", got.Tasks[0].Confidence)
	}
	want := []CrossSignal{{Pattern: "alpha", Slices: []string{"s1", "s2"}}, {Pattern: "zeta", Slices: []string{"s1", "s2"}}}
	if !reflect.DeepEqual(got.CrossCutting, want) {
		t.Errorf("cross-cutting = %+v, want %+v (sorted by pattern)", got.CrossCutting, want)
	}
}

func TestCoverfixPartitionPackagesClampsAnOutOfRangeSliceIndex(t *testing.T) {
	saved := sliceIndex
	t.Cleanup(func() { sliceIndex = saved })
	sliceIndex = func(i, per int) int { return 99 }
	got := partitionPackages([]string{"x/a", "x/b", "x/c"}, 2)
	if len(got) != 1 || got[0].ID != "slice-2" || !reflect.DeepEqual(got[0].Packages, []string{"x/a", "x/b", "x/c"}) {
		t.Errorf("out-of-range index must land in the last slice, got %+v", got)
	}
}
