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
				// same slug+files as slice-1's fix-nil ⇒ dedups, occurrences=2, conf=max
				{Slug: "fix-nil", Type: "stability", Complexity: "S", Files: []string{"a.go"}, Confidence: 0.9},
			},
			CrossSliceSignals: []CrossSignal{{Pattern: "os.ReadFile+json.Unmarshal"}}, // same pattern in 2 slices
		},
	}

	var m MergedFindings = mergeDigests(digests)

	// dedup: fix-nil collapses to one task, occurrences=2, confidence=max(0.6,0.9)=0.9
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
	// single-slice task ranks below the 2-occurrence one despite irrelevant conf
	if m.Tasks[1].Slug != "dedup-json" || m.Tasks[1].Occurrences != 1 {
		t.Errorf("second task = %+v, want dedup-json occ=1", m.Tasks[1])
	}
	// cross-cutting: pattern in >=2 slices is confirmed
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

// A pattern flagged by only ONE slice is not cross-cutting.
func TestMergeDigests_SingleSliceSignalNotCrossCutting(t *testing.T) {
	t.Parallel()
	m := mergeDigests([]ScanDigest{
		{SliceID: "slice-1", CrossSliceSignals: []CrossSignal{{Pattern: "lonely"}}},
	})
	if len(m.CrossCutting) != 0 {
		t.Errorf("single-slice signal must not be cross-cutting; got %+v", m.CrossCutting)
	}
}
