package scout

import "testing"

func TestMergeDigests(t *testing.T) {
	t.Parallel()
	digests := []ScanDigest{
		{
			SliceID:  "slice-1",
			Findings: []Finding{{File: "a.go", Kind: "gap", Severity: "HIGH", Note: "nil deref"}},
			CandidateTasks: []CandidateTask{
				{Slug: "fix-nil", Type: "stability", Complexity: "S", Files: []string{"a.go"}, Confidence: 0.6},
				{Slug: "dedup-json", Type: "debt", Complexity: "M", Files: []string{"a.go"}, Confidence: 0.5},
			},
			CrossSliceSignals: []CrossSignal{{Pattern: "os.ReadFile+json.Unmarshal"}},
		},
		{
			SliceID:  "slice-2",
			Findings: []Finding{{File: "b.go", Kind: "debt", Severity: "MEDIUM", Note: "dup"}},
			CandidateTasks: []CandidateTask{
				{Slug: "fix-nil", Type: "stability", Complexity: "S", Files: []string{"a.go"}, Confidence: 0.9},
			},
			CrossSliceSignals: []CrossSignal{{Pattern: "os.ReadFile+json.Unmarshal"}},
		},
	}

	var m MergedFindings = mergeDigests(digests)

	if len(m.Tasks) != 2 {
		t.Fatalf("want 2 deduped tasks, got %d: %+v", len(m.Tasks), m.Tasks)
	}
	if m.Tasks[0].Slug != "fix-nil" {
		t.Errorf("cross-slice-agreed task must rank first; got %q", m.Tasks[0].Slug)
	}
	if m.Tasks[0].Occurrences != 2 {
		t.Errorf("fix-nil occurrences=%d, want 2", m.Tasks[0].Occurrences)
	}
	if m.Tasks[0].Confidence != 0.9 {
		t.Errorf("fix-nil confidence=%.2f, want 0.9 (max)", m.Tasks[0].Confidence)
	}
	if m.Tasks[1].Slug != "dedup-json" || m.Tasks[1].Occurrences != 1 {
		t.Errorf("second task = %+v, want dedup-json occ=1", m.Tasks[1])
	}
	if len(m.CrossCutting) != 1 || m.CrossCutting[0].Pattern != "os.ReadFile+json.Unmarshal" {
		t.Errorf("pattern flagged by 2 slices must be cross-cutting; got %+v", m.CrossCutting)
	}
	if len(m.CrossCutting[0].Slices) != 2 {
		t.Errorf("cross-cutting must name both slices; got %v", m.CrossCutting[0].Slices)
	}
	if len(m.Findings) != 2 {
		t.Errorf("all findings collected; want 2, got %d", len(m.Findings))
	}
}

func TestMergeDigests_SingleSliceSignalNotCrossCutting(t *testing.T) {
	t.Parallel()
	m := mergeDigests([]ScanDigest{
		{SliceID: "slice-1", CrossSliceSignals: []CrossSignal{{Pattern: "lonely"}}},
	})
	if len(m.CrossCutting) != 0 {
		t.Errorf("single-slice signal must not be cross-cutting; got %+v", m.CrossCutting)
	}
}
