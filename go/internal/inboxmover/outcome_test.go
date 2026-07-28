package inboxmover

// outcome_test.go — unit coverage for the cycle-outcome seam's readers and
// result surface. The end-to-end filesystem lifecycle (PASS-promote,
// FAIL-bump, quarantine-at-ceiling, menu semantics) is pinned by the cycle-1156
// ACS predicates; these tests cover the parsing and reporting edges those
// predicates deliberately do not assert on.

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestCommittedIDs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
		want []string
	}{
		{"invalid json", "not json{", nil},
		{"empty object", "{}", nil},
		{
			"top_n union skip_shipped, deduped and order-preserving",
			`{"top_n":[{"id":"a"},{"id":""},{"id":"b"},{"id":"a"}],"skip_shipped":[{"task_id":"b"},{"task_id":"c"}]}`,
			[]string{"a", "b", "c"},
		},
		{"skip_shipped only", `{"skip_shipped":[{"task_id":"z"}]}`, []string{"z"}},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := CommittedIDs([]byte(tc.body))
			if len(got) != len(tc.want) {
				t.Fatalf("CommittedIDs(%s) = %v, want %v", tc.body, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("CommittedIDs(%s)[%d] = %q, want %q", tc.body, i, got[i], tc.want[i])
				}
			}
		})
	}
}

// TestApplyCycleOutcome_ResultReportsMovedIDs asserts the OutcomeResult surface:
// a PASS names the ids it actually promoted, and re-applying names none (the
// idempotent no-op reports nothing moved rather than double-counting).
func TestApplyCycleOutcome_ResultReportsMovedIDs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inbox, "item-a.json"), []byte(`{"id":"a"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := Options{ProjectRoot: root, Stderr: io.Discard, IsLandedFn: func(string) (bool, error) { return true, nil }}
	oc := CycleOutcome{Cycle: 7, Passed: true, CommittedIDs: []string{"a", "a", ""}}

	var res OutcomeResult
	res, err := ApplyCycleOutcome(opts, oc)
	if err != nil {
		t.Fatalf("ApplyCycleOutcome: %v", err)
	}
	if len(res.Promoted) != 1 || res.Promoted[0] != "a" {
		t.Errorf("res.Promoted = %v, want [a] (duplicate and empty ids collapse)", res.Promoted)
	}

	res, err = ApplyCycleOutcome(opts, oc)
	if err != nil {
		t.Fatalf("second ApplyCycleOutcome: %v", err)
	}
	if len(res.Promoted) != 0 {
		t.Errorf("res.Promoted = %v on re-apply, want empty: an already-promoted id moved nothing", res.Promoted)
	}
}

// TestClaimLaneScope_ToleratesUnresolvableIDs pins the partial-claim contract:
// an id that cannot be claimed is skipped, never fatal — a lane must not abort
// because one item of its scope moved on.
func TestClaimLaneScope_ToleratesUnresolvableIDs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inbox, "item-a.json"), []byte(`{"id":"a"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	claimed, err := ClaimLaneScope(Options{ProjectRoot: root, Stderr: io.Discard}, 7, []string{"a", "ghost"})
	if err != nil {
		t.Fatalf("ClaimLaneScope: %v", err)
	}
	if len(claimed) != 1 || claimed[0] != "a" {
		t.Fatalf("claimed = %v, want [a]", claimed)
	}
	if _, statErr := os.Stat(filepath.Join(inbox, "processing", "cycle-7", "item-a.json")); statErr != nil {
		t.Errorf("item-a.json not claimed into processing/cycle-7/: %v", statErr)
	}
}
