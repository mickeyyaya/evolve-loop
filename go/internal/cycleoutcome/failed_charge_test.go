package cycleoutcome

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
)

func TestApplyFailure_ChargesOnlyThePinMembersTheDecisionNames(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		pin      []string
		decision string
		want     map[string]int
	}{
		{"an unmentioned menu mate is not charged", []string{"a", "b"}, `{"top_n":[{"id":"a"}]}`, map[string]int{"a": 1, "b": 0}},
		{"a deferred pin is owed and charged", []string{"a", "b"}, `{"top_n":[],"deferred":[{"id":"a"}]}`, map[string]int{"a": 1, "b": 1}},
		{"a deferred mate stays uncharged beside a named one", []string{"a", "b"}, `{"top_n":[{"id":"a"}],"deferred":[{"id":"b"}]}`, map[string]int{"a": 1, "b": 0}},
		{"a lone deferred pin is charged", []string{"a"}, `{"top_n":[],"deferred":[{"id":"a"}]}`, map[string]int{"a": 1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			inbox := filepath.Join(root, ".evolve", "inbox")
			for _, id := range tc.pin {
				writeLaneItem(t, inbox, id, "go/internal/"+id+"/"+id+".go")
			}
			ws := filepath.Join(root, ".evolve", "runs", "cycle-12")
			if err := os.MkdirAll(ws, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := committedset.WriteRecord(root, committedset.Record{Cycle: 12, ItemIDs: tc.pin}); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(ws, "triage-decision.json"), []byte(tc.decision), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := ApplyFailure(FailureInputs{ProjectRoot: root, Workspace: ws, Cycle: 12, Ceiling: 3, Stderr: io.Discard}); err != nil {
				t.Fatalf("ApplyFailure: %v", err)
			}
			for id, want := range tc.want {
				if got := itemFailureCount(t, findItemFile(t, inbox, id)); got != want {
					t.Errorf("item %s failure_count = %d, want %d", id, got, want)
				}
			}
		})
	}
}
